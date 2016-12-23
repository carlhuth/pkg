// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signed

import (
	"bytes"
	"encoding/hex"
	"net/http"

	"github.com/corestoreio/csfw/util/bufferpool"
	"github.com/corestoreio/errors"
)

const signatureDefaultSeparator = ','

// ContentSignature represents an HTTP Header or Trailer entry with the default header
// key Content-Signature.
type ContentSignature struct {
	// KeyID field is an opaque string that the server/client can use to look up
	// the component they need to validate the signature. It could be an SSH key
	// fingerprint, an LDAP DN, etc. REQUIRED.
	KeyID string
	// Separator defines the field separator and defaults to colon.
	Separator rune
	ContentHMAC
}

// NewContentSignature creates a new header signature object with default hex
// encoding/decoding to write and parse the Content-Signature field.
func NewContentSignature(keyID, algorithm string) *ContentSignature {
	return &ContentSignature{
		KeyID: keyID,
		ContentHMAC: ContentHMAC{
			Algorithm: algorithm,
		},
	}
}

// HeaderKey returns the name of the header key
func (s *ContentSignature) HeaderKey() string {
	if s.HeaderName != "" {
		return s.HeaderName
	}
	return HeaderContentSignature
}

// Write writes the content signature header using an
// encoder, which can be hex or base64.
//
// Signature parameter is an encoded digital signature generated by the
// client.  The client uses the `algorithm` and `headers` request parameters
// to form a canonicalized `signing string`.  This `signing string` is then
// signed with the key associated with `keyId` and the algorithm
// corresponding to `algorithm`.  The `signature` parameter is then set to
// the encoding of the signature.
//
// 	Content-Signature: keyId="rsa-key-1",algorithm="rsa-sha256",signature="Hex|Base64(RSA-SHA256(signing string))"
// 	Content-Signature: keyId="hmac-key-1",algorithm="hmac-sha1",signature="Hex|Base64(HMAC-SHA1(signing string))"
func (s *ContentSignature) Write(w http.ResponseWriter, signature []byte) {
	if s.Separator == 0 {
		s.Separator = signatureDefaultSeparator
	}

	encFn := s.EncodeFn
	if encFn == nil {
		encFn = hex.EncodeToString
	}
	buf := bufferpool.Get()
	_, _ = buf.Write(prefixKeyID)
	_, _ = buf.WriteString(s.KeyID)
	_, _ = buf.Write(suffixQuote)
	_, _ = buf.WriteRune(s.Separator)
	_, _ = buf.Write(prefixAlgorithm)
	_, _ = buf.WriteString(s.Algorithm)
	_, _ = buf.Write(suffixQuote)
	_, _ = buf.WriteRune(s.Separator)
	_, _ = buf.Write(prefixSignature)
	_, _ = buf.WriteString(encFn(signature))
	_, _ = buf.Write(suffixQuote)
	w.Header().Set(s.HeaderKey(), buf.String())
	bufferpool.Put(buf)
}

// Parse looks up the header or trailer for the HeaderKey Content-Signature in an
// HTTP request and extracts the raw decoded signature. Errors can have the
// behaviour: NotFound or NotValid.
func (s *ContentSignature) Parse(r *http.Request) (signature []byte, _ error) {
	if s.Separator == 0 {
		s.Separator = signatureDefaultSeparator
	}
	k := s.HeaderKey()
	headerVal := r.Header.Get(k)
	if headerVal == "" {
		headerVal = r.Trailer.Get(k)
	}
	if headerVal == "" {
		return nil, errors.NewNotFoundf(errSignatureParseNotFound)
	}

	// keyId="hmac-key-1",algorithm="hmac-sha1",signature="Hex|Base64(HMAC-SHA1(signing string))"

	var fields [3]bytes.Buffer
	var idx int
	for _, r := range headerVal {
		if r == s.Separator {
			idx++
			continue
		}
		if idx > 2 { // too many separators
			return nil, errors.NewNotValidf(errSignatureParseInvalidHeader, headerVal)
		}
		_, _ = fields[idx].WriteRune(r)
	}
	if idx < 2 { // too less separators
		return nil, errors.NewNotValidf(errSignatureParseInvalidHeader, headerVal)
	}

	// trim first and last white spaces
	for i := 0; i < 3; i++ {
		tmp := fields[i].Bytes()
		fields[i].Reset()
		_, _ = fields[i].Write(bytes.TrimSpace(tmp))
	}

	// check prefix and suffix
	switch {
	case !bytes.HasPrefix(fields[0].Bytes(), prefixKeyID) || !bytes.HasSuffix(fields[0].Bytes(), suffixQuote): // keyId="..."
		return nil, errors.NewNotValidf("[signed] keyId %q missing suffix %q or prefix %q in header: %q", fields[0].Bytes(), prefixKeyID, suffixQuote, headerVal)
	case !bytes.HasPrefix(fields[1].Bytes(), prefixAlgorithm) || !bytes.HasSuffix(fields[1].Bytes(), suffixQuote): // algorithm="..."
		return nil, errors.NewNotValidf("[signed] algorithm %q missing suffix %q or prefix %q in header: %q", fields[1].Bytes(), prefixAlgorithm, suffixQuote, headerVal)
	case !bytes.HasPrefix(fields[2].Bytes(), prefixSignature) || !bytes.HasSuffix(fields[2].Bytes(), suffixQuote): // signature="..."
		return nil, errors.NewNotValidf("[signed] signature %q missing suffix %q or prefix %q in header: %q", fields[2].Bytes(), prefixSignature, suffixQuote, headerVal)
	}

	// check for valid keyID
	if haveKeyID := fields[0].String()[7 : fields[0].Len()-1]; s.KeyID != haveKeyID || s.KeyID == "" {
		return nil, errors.NewNotValidf(errSignatureParseInvalidKeyID, haveKeyID, s.KeyID, headerVal)
	}

	// check for valid algorithm
	if haveAlg := fields[1].String()[11 : fields[1].Len()-1]; s.Algorithm != haveAlg || s.Algorithm == "" {
		return nil, errors.NewNotValidf(errSignatureParseInvalidAlg, haveAlg, s.Algorithm, headerVal)
	}

	decFn := s.DecodeFn
	if decFn == nil {
		decFn = hex.DecodeString
	}
	rawSig := fields[2].String()[11 : fields[2].Len()-1]
	dec, err := decFn(rawSig)
	if err != nil {
		// micro optimization: skip argument building
		return nil, errors.Wrapf(err, "[signed] failed to decode: %q in header %q", rawSig, headerVal)
	}
	return dec, nil
}

var (
	prefixKeyID     = []byte(`keyId="`)
	prefixAlgorithm = []byte(`algorithm="`)
	prefixSignature = []byte(`signature="`)
	suffixQuote     = []byte(`"`)
)
