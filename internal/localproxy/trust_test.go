package localproxy

import "testing"

func TestCertificateOutputHasSHA256(t *testing.T) {
	out := []byte(`
keychain: "/Users/example/Library/Keychains/login.keychain-db"
version: 512
class: 0x80001000
attributes:
    "alis"<blob>="scenery Development Local Proxy CA"
SHA-256 hash: 0261A834E2A7B25A191A62EC6197C7A6EBD41A77509657808D33B59CF97152D4
-----BEGIN CERTIFICATE-----
MIIB
-----END CERTIFICATE-----
`)
	if !certificateOutputHasSHA256(out, "02:61:A8:34:E2:A7:B2:5A:19:1A:62:EC:61:97:C7:A6:EB:D4:1A:77:50:96:57:80:8D:33:B5:9C:F9:71:52:D4") {
		t.Fatal("certificateOutputHasSHA256() did not match fingerprint with separators")
	}
	if certificateOutputHasSHA256(out, "0261A834E2A7B25A191A62EC6197C7A6EBD41A77509657808D33B59CF97152D5") {
		t.Fatal("certificateOutputHasSHA256() matched a different fingerprint")
	}
}
