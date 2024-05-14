package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPutOnTheBottomOfView(t *testing.T) {
	assert.Equal(t, "", PutOnTheBottomOfView("", "", 0))
	assert.Equal(t, "1", PutOnTheBottomOfView("1", "2", 0))
	assert.Equal(t, "1", PutOnTheBottomOfView("1", "2", 1))
	assert.Equal(t, "1\n2", PutOnTheBottomOfView("1", "2", 2))
	assert.Equal(t, "1\n\n2", PutOnTheBottomOfView("1", "2", 3))
	assert.Equal(t, "1\n2\n3", PutOnTheBottomOfView("1\n2", "3", 3))
	assert.Equal(t, "1\n2\n", PutOnTheBottomOfView("1\n2\n", "3", 3))
	assert.Equal(t, "1\n2\n\n3", PutOnTheBottomOfView("1\n2\n", "3", 4))
	assert.Equal(t, "\n\n3\n4", PutOnTheBottomOfView("", "3\n4", 4))
	assert.Equal(t, "\n3\n4\n", PutOnTheBottomOfView("", "3\n4\n", 4))
}
