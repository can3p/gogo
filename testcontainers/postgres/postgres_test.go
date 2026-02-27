package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	code := m.Run()
	_ = Cleanup()
	if code != 0 {
		panic(code)
	}
}

func TestNewTestDB(t *testing.T) {
	testDB, err := NewTestDB(Options{})
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	var result int
	err = testDB.DB.Get(&result, "SELECT 1")
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestConnectionInfo(t *testing.T) {
	testDB, err := NewTestDB(Options{})
	require.NoError(t, err)
	defer func() { _ = testDB.Close() }()

	assert.NotEmpty(t, testDB.ConnInfo.Host)
	assert.NotEmpty(t, testDB.ConnInfo.Port)
	assert.Equal(t, "testuser", testDB.ConnInfo.User)
	assert.Equal(t, "secret", testDB.ConnInfo.Password)
	assert.NotEmpty(t, testDB.ConnInfo.DBName)
	assert.Equal(t, "disable", testDB.ConnInfo.SSLMode)

	connStr := testDB.ConnInfo.ConnectionString()
	assert.Contains(t, connStr, "postgres://")
	assert.Contains(t, connStr, testDB.ConnInfo.DBName)
}

func TestParallelDatabases(t *testing.T) {
	t.Run("db1", func(t *testing.T) {
		t.Parallel()
		testDB, err := NewTestDB(Options{})
		require.NoError(t, err)
		defer func() { _ = testDB.Close() }()

		_, err = testDB.DB.Exec("CREATE TABLE test_table (id INT PRIMARY KEY)")
		require.NoError(t, err)

		var count int
		err = testDB.DB.Get(&count, "SELECT COUNT(*) FROM test_table")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("db2", func(t *testing.T) {
		t.Parallel()
		testDB, err := NewTestDB(Options{})
		require.NoError(t, err)
		defer func() { _ = testDB.Close() }()

		// This database should not have test_table from db1
		var exists bool
		err = testDB.DB.Get(&exists, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
