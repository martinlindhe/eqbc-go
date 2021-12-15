package eqbc

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLoginPacket(t *testing.T) {
	user, pwd, err := parseLoginPacket([]byte("LOGIN=name;"))
	assert.Equal(t, nil, err)
	assert.Equal(t, "name", user)
	assert.Equal(t, "", pwd)

	user, pwd, err = parseLoginPacket([]byte("LOGIN:pwd=name;"))
	assert.Equal(t, nil, err)
	assert.Equal(t, "name", user)
	assert.Equal(t, "pwd", pwd)
}

func TestColorize(t *testing.T) {
	s := "[+g+]green [+y+]yellow [+r+]red"
	fmt.Println(colorize(s))
}
