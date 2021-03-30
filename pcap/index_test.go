package pcap_test

import (
	"strings"
	"testing"

	"github.com/brimsec/zq/pcap"
	"github.com/brimsec/zq/pcap/pcapio"
	"github.com/stretchr/testify/require"
)

func TestInvalidIndex(t *testing.T) {
	r := strings.NewReader("this is not a valid pcap.")
	_, err := pcap.CreateIndex(r, 0)
	require.ErrorIs(t, err, &pcapio.ErrInvalidPcap{})
}
