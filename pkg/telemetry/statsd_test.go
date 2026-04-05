package telemetry

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetric_Success(t *testing.T) {
	t.Parallel()

	t.Run("Nil Metric", func(t *testing.T) {
		var m *Metric = nil
		assert.Nil(t, nil, m.Status("success"))
		m.Publish() // should not panic.
	})

	t.Run("Simple Use", func(t *testing.T) {
		m := Metric{
			name: "foo",
			publishFunc: func(name string, tags []string) error {
				assert.Equal(t, "foo", name)
				assert.Equal(t, []string{"status:success"}, tags)
				return nil
			},
		}
		m.Publish()
	})

	t.Run("Explicit Success", func(t *testing.T) {
		m := Metric{
			name: "foo",
			publishFunc: func(name string, tags []string) error {
				assert.Equal(t, "foo", name)
				assert.Equal(t, []string{"status:success"}, tags)
				return nil
			},
		}
		m.Status("success").Publish()
	})

	t.Run("Explicit Failure", func(t *testing.T) {
		m := Metric{
			name: "foo",
			publishFunc: func(name string, tags []string) error {
				assert.Equal(t, "foo", name)
				assert.Equal(t, []string{"status:failed"}, tags)
				return nil
			},
		}
		m.Status("failed").Publish()
	})

	t.Run("With Custom Tags", func(t *testing.T) {
		m := Metric{
			name: "foo",
			publishFunc: func(name string, tags []string) error {
				// sorting to avoid random ordered tags.
				sort.Slice(tags, func(i, j int) bool {
					return strings.Compare(tags[i], tags[j]) < 0
				})

				assert.Equal(t, "foo", name)

				assert.Equal(t, []string{"method:bar", "status:success"}, tags)
				return nil
			},
		}
		m.
			Tag("method", "bar").
			Status("success").
			Publish()
	})
}
