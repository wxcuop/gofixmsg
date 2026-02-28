package crypt_test

import (
"testing"

"github.com/stretchr/testify/require"
"github.com/wxcuop/pyfixmsg_plus/crypt"
)

func TestEncryptDecrypt(t *testing.T) {
pass := "s3cr3t"
plain := "my very secret value"
enc, err := crypt.EncryptString(plain, pass)
require.NoError(t, err)
got, err := crypt.DecryptString(enc, pass)
require.NoError(t, err)
require.Equal(t, plain, got)
}
