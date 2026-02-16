package bramble

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestBuildSubscriptionRegistry(t *testing.T) {
	t.Run("extracts topic pattern from @topic directive", func(t *testing.T) {
		schema := gqlparser.MustLoadSchema(&ast.Source{
			Name: "test",
			Input: `
				directive @topic(pattern: String!) on FIELD_DEFINITION

				type PoolUpdate {
					poolId: ID!
					quantityA: String!
				}

				type Query {
					dummy: String!
				}

				type Subscription {
					poolUpdated(id: ID!): PoolUpdate! @topic(pattern: "pool:{{id}}")
				}
			`,
		})

		locations := FieldURLMap{}
		locations.RegisterURL("Subscription", "poolUpdated", "http://protocol:8080")
		services := map[string]*Service{
			"http://protocol:8080": {Name: "sundae-protocol", ServiceURL: "http://protocol:8080"},
		}

		reg, err := BuildSubscriptionRegistry(schema, map[string]bool{}, services, locations)
		require.NoError(t, err)
		require.Len(t, reg.Fields, 1)

		field := reg.Fields["poolUpdated"]
		assert.Equal(t, "poolUpdated", field.Name)
		assert.Equal(t, "pool:{{id}}", field.TopicPattern)
		assert.Equal(t, "PoolUpdate", field.ReturnTypeName)
		assert.Equal(t, "sundae-protocol", field.SourceService)
		assert.Equal(t, "ID!", field.Args["id"])
	})

	t.Run("rejects boundary return types", func(t *testing.T) {
		schema := gqlparser.MustLoadSchema(&ast.Source{
			Name: "test",
			Input: `
				directive @boundary on OBJECT
				directive @topic(pattern: String!) on FIELD_DEFINITION

				type Pool @boundary {
					id: ID!
				}

				type Query {
					dummy: String!
				}

				type Subscription {
					poolUpdated(id: ID!): Pool! @topic(pattern: "pool:{{id}}")
				}
			`,
		})

		_, err := BuildSubscriptionRegistry(schema, map[string]bool{"Pool": true}, map[string]*Service{}, FieldURLMap{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boundary type")
		assert.Contains(t, err.Error(), "Pool")
	})

	t.Run("handles no subscription type", func(t *testing.T) {
		schema := gqlparser.MustLoadSchema(&ast.Source{
			Name: "test",
			Input: `
				type Query {
					dummy: String!
				}
			`,
		})

		reg, err := BuildSubscriptionRegistry(schema, map[string]bool{}, map[string]*Service{}, FieldURLMap{})
		require.NoError(t, err)
		assert.Empty(t, reg.Fields)
	})

	t.Run("handles field without @topic directive", func(t *testing.T) {
		schema := gqlparser.MustLoadSchema(&ast.Source{
			Name: "test",
			Input: `
				type Heartbeat {
					timestamp: Int!
				}

				type Query {
					dummy: String!
				}

				type Subscription {
					heartbeat: Heartbeat!
				}
			`,
		})

		reg, err := BuildSubscriptionRegistry(schema, map[string]bool{}, map[string]*Service{}, FieldURLMap{})
		require.NoError(t, err)
		require.Len(t, reg.Fields, 1)
		assert.Equal(t, "", reg.Fields["heartbeat"].TopicPattern)
	})
}

func TestComputeTopic(t *testing.T) {
	reg := &SubscriptionRegistry{
		Fields: map[string]SubscriptionField{
			"poolUpdated": {
				Name:         "poolUpdated",
				TopicPattern: "pool:{{id}}",
				Args:         map[string]string{"id": "ID!"},
			},
			"orderUpdated": {
				Name:         "orderUpdated",
				TopicPattern: "order:{{poolId}}:{{status}}",
				Args:         map[string]string{"poolId": "ID!", "status": "String!"},
			},
			"heartbeat": {
				Name:         "heartbeat",
				TopicPattern: "",
			},
		},
	}

	t.Run("substitutes single placeholder", func(t *testing.T) {
		topic, err := reg.ComputeTopic("poolUpdated", map[string]interface{}{"id": "abc123"})
		require.NoError(t, err)
		assert.Equal(t, "pool:abc123", topic)
	})

	t.Run("substitutes multiple placeholders", func(t *testing.T) {
		topic, err := reg.ComputeTopic("orderUpdated", map[string]interface{}{
			"poolId": "pool1",
			"status": "filled",
		})
		require.NoError(t, err)
		assert.Equal(t, "order:pool1:filled", topic)
	})

	t.Run("errors on unknown field", func(t *testing.T) {
		_, err := reg.ComputeTopic("nonexistent", map[string]interface{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown subscription field")
	})

	t.Run("errors on empty topic pattern", func(t *testing.T) {
		_, err := reg.ComputeTopic("heartbeat", map[string]interface{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no @topic pattern")
	})

	t.Run("errors on unresolved placeholders", func(t *testing.T) {
		_, err := reg.ComputeTopic("poolUpdated", map[string]interface{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unresolved placeholders")
	})
}

func TestValidateField(t *testing.T) {
	reg := &SubscriptionRegistry{
		Fields: map[string]SubscriptionField{
			"poolUpdated": {Name: "poolUpdated"},
		},
	}

	t.Run("valid field", func(t *testing.T) {
		err := reg.ValidateField("poolUpdated")
		assert.NoError(t, err)
	})

	t.Run("unknown field", func(t *testing.T) {
		err := reg.ValidateField("nonexistent")
		assert.Error(t, err)
	})
}
