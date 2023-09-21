package probes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgreSQL_Probe(t *testing.T) {
	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	user := "foo"
	password := "bar"
	db := "buzz"

	res, err := pool.Run("postgres", "12.3", []string{
		"POSTGRES_USER=" + user,
		"POSTGRES_PASSWORD=" + password,
		"POSTGRES_DB=" + db,
	})
	require.NoError(t, err)
	defer pool.Purge(res)

	var p Prober
	assert.Eventually(t, func() bool {
		p, err = NewPostgreSQL("VSHN", "test", "test",
			fmt.Sprintf(
				"postgresql://%s:%s@%s:%s/%s?sslmode=disable",
				user,
				password,
				"localhost",
				res.GetPort("5432/tcp"),
				db,
			),
			"test", "besteffort", 1,
		)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		err = p.Probe(context.TODO())
		return err == nil
	}, 10*time.Second, 500*time.Millisecond)
	assert.NoError(t, err)

	assert.Never(t, func() bool {
		err = p.Probe(context.TODO())
		return err != nil
	}, 1*time.Second, 100*time.Millisecond)

	require.NoError(t, res.Close())

	assert.Eventually(t, func() bool {
		err = p.Probe(context.TODO())
		return err != nil
	}, 2*time.Second, 500*time.Millisecond)
	assert.Error(t, err)

	assert.Never(t, func() bool {
		err = p.Probe(context.TODO())
		return err == nil
	}, 1*time.Second, 100*time.Millisecond)

	assert.NoError(t, p.Close())

}

func TestPostgreSQL_Fail(t *testing.T) {
	t.Parallel()
	p, err := NewFailingPostgreSQL("FAKE", "foo", "bar")
	require.NoError(t, err)

	require.NotPanics(t, func() {
		assert.Error(t, p.Probe(context.Background()))

		pi := p.GetInfo()
		assert.Equal(t, pi.Service, "FAKE")
		assert.Equal(t, pi.Name, "foo")
		assert.Equal(t, pi.Namespace, "bar")
		assert.NoError(t, p.Close())
	})
}
