package bramble

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// SubscriptionField describes a single federated subscription field.
type SubscriptionField struct {
	Name           string
	TopicPattern   string
	Args           map[string]string // arg name → GraphQL type string
	ReturnTypeName string
	SourceService  string
}

// SubscriptionRegistry holds the set of subscription fields extracted from the
// merged schema. It is rebuilt every time the schema is refreshed.
type SubscriptionRegistry struct {
	Fields map[string]SubscriptionField
}

// BuildSubscriptionRegistry iterates over the merged Subscription type and
// extracts @topic metadata from each field. It validates that no subscription
// return type is a @boundary type (cross-service resolution is not supported
// for subscriptions in v1).
func BuildSubscriptionRegistry(schema *ast.Schema, isBoundary map[string]bool, services map[string]*Service, locations FieldURLMap) (*SubscriptionRegistry, error) {
	reg := &SubscriptionRegistry{
		Fields: make(map[string]SubscriptionField),
	}

	subType := schema.Subscription
	if subType == nil {
		return reg, nil
	}

	for _, field := range subType.Fields {
		if isGraphQLBuiltinName(field.Name) {
			continue
		}

		returnTypeName := field.Type.Name()

		// Reject subscription fields whose return type is a @boundary type
		if isBoundary[returnTypeName] {
			return nil, fmt.Errorf(
				"subscription field %q returns boundary type %q; subscription return types must be fully owned by the declaring service",
				field.Name, returnTypeName,
			)
		}

		// Extract @topic directive
		topicDir := field.Directives.ForName(topicDirectiveName)
		var topicPattern string
		if topicDir != nil {
			if patternArg := topicDir.Arguments.ForName("pattern"); patternArg != nil {
				topicPattern = patternArg.Value.Raw
			}
		}

		// Build args map
		args := make(map[string]string, len(field.Arguments))
		for _, arg := range field.Arguments {
			args[arg.Name] = arg.Type.String()
		}

		// Determine source service from field URL map
		sourceService := ""
		key := locations.keyFor(subscriptionObjectName, field.Name)
		if url, ok := locations[key]; ok && url != "" {
			if svc, ok := services[url]; ok {
				sourceService = svc.Name
			}
		}

		reg.Fields[field.Name] = SubscriptionField{
			Name:           field.Name,
			TopicPattern:   topicPattern,
			Args:           args,
			ReturnTypeName: returnTypeName,
			SourceService:  sourceService,
		}
	}

	return reg, nil
}

// ComputeTopic substitutes {{argName}} placeholders in the topic pattern with
// the provided argument values.
func (r *SubscriptionRegistry) ComputeTopic(fieldName string, args map[string]interface{}) (string, error) {
	sf, ok := r.Fields[fieldName]
	if !ok {
		return "", fmt.Errorf("unknown subscription field %q", fieldName)
	}

	if sf.TopicPattern == "" {
		return "", fmt.Errorf("subscription field %q has no @topic pattern", fieldName)
	}

	topic := sf.TopicPattern
	for argName, argVal := range args {
		placeholder := "{{" + argName + "}}"
		topic = strings.ReplaceAll(topic, placeholder, fmt.Sprintf("%v", argVal))
	}

	// Check for unresolved placeholders
	if strings.Contains(topic, "{{") {
		return "", fmt.Errorf("unresolved placeholders in topic pattern %q for field %q", topic, fieldName)
	}

	return topic, nil
}

// ValidateField checks that the given field name exists in the registry.
func (r *SubscriptionRegistry) ValidateField(fieldName string) error {
	if _, ok := r.Fields[fieldName]; !ok {
		return fmt.Errorf("unknown subscription field %q", fieldName)
	}
	return nil
}
