package tlsconfig

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
)

const pemCertificateType = "CERTIFICATE"

// An encapsulation boundary opens a PEM block. pem.Decode recognizes it at the start of
// the data or at the start of a line, and countBoundaries follows the same rule.
var (
	pemBoundary     = []byte("-----BEGIN ")
	pemLineBoundary = []byte("\n-----BEGIN ")
)

var errMalformedPEMBlock = errors.New("malformed PEM block")

// decodePEMBlocks returns the contents of every PEM block in data, all of which must be of
// blockType. Text outside encapsulation boundaries is explanatory and ignored (RFC 7468,
// section 2), so a file that keeps the human-readable dump of `openssl x509 -text` or
// `openssl crl -text` in front of each block loads. pem.Decode also skips a block it cannot
// decode; that is reported as malformed rather than treated as text, so a truncated or
// corrupted file fails to load instead of quietly supplying less trust material than it
// describes.
func decodePEMBlocks(data []byte, blockType string) ([][]byte, error) {
	var blocks [][]byte
	rest := data
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			if countBoundaries(rest) > 0 {
				return nil, errMalformedPEMBlock
			}

			return blocks, nil
		}
		// The consumed text holds the boundary of the decoded block; any other boundary in
		// it opened a block that pem.Decode skipped.
		if countBoundaries(rest[:len(rest)-len(next)]) > 1 {
			return nil, errMalformedPEMBlock
		}
		if block.Type != blockType {
			return nil, fmt.Errorf("unexpected %q PEM block, want %q", block.Type, blockType)
		}

		blocks = append(blocks, block.Bytes)
		rest = next
	}
}

func countBoundaries(data []byte) int {
	count := bytes.Count(data, pemLineBoundary)
	if bytes.HasPrefix(data, pemBoundary) {
		count++
	}

	return count
}
