package bitwarden

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dortanes/ravenpass/packages/importers"
)

// The fastest derivations Bitwarden's ranges allow.
var (
	fastPBKDF2   = derivation{function: kdfPBKDF2, iterations: minPBKDF2Iterations}
	fastArgon2id = derivation{function: kdfArgon2id, iterations: minArgon2Iterations, memoryMiB: minArgon2MemoryMiB, parallelism: minArgon2Parallelism}
)

const exportSalt = "eW91ci1zYWx0|as written"

func keysFor(t *testing.T, password string, scheme derivation, salt string) keys {
	t.Helper()
	material, err := scheme.derive(password, []byte(salt))
	if err != nil {
		t.Fatal(err)
	}
	stretched, err := stretch(material)
	if err != nil {
		t.Fatal(err)
	}
	return stretched
}

// sealBlocks encrypts whole blocks without padding them and writes the EncString.
func sealBlocks(t *testing.T, k keys, blocks []byte) string {
	t.Helper()
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(k.encryption)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(blocks))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, blocks)
	tag := hmac.New(sha256.New, k.authentication)
	tag.Write(iv)
	tag.Write(ciphertext)
	encode := base64.StdEncoding.EncodeToString
	return encStringType + "." + encode(iv) + "|" + encode(ciphertext) + "|" + encode(tag.Sum(nil))
}

func seal(t *testing.T, k keys, plaintext []byte) string {
	t.Helper()
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	return sealBlocks(t, k, append(bytes.Clone(plaintext), bytes.Repeat([]byte{byte(padding)}, padding)...))
}

// protectedExport writes a password-protected export of plaintext as Bitwarden lays it out.
func protectedExport(t *testing.T, password string, scheme derivation, plaintext []byte) []byte {
	t.Helper()
	k := keysFor(t, password, scheme, exportSalt)
	defer k.wipe()
	document, err := json.Marshal(map[string]any{
		"encrypted":                    true,
		"passwordProtected":            true,
		"salt":                         exportSalt,
		"kdfType":                      int(scheme.function),
		"kdfIterations":                scheme.iterations,
		"kdfMemory":                    scheme.memoryMiB,
		"kdfParallelism":               scheme.parallelism,
		"encKeyValidation_DO_NOT_EDIT": seal(t, k, []byte("0b1f6e4c-5d1a-4a53-9a57-3a8f1c2e7d90")),
		"data":                         seal(t, k, plaintext),
	})
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestDerivationMatchesTheBitwardenVectors(t *testing.T) {
	tests := []struct {
		name   string
		scheme derivation
		want   []byte
	}{
		{"PBKDF2", derivation{function: kdfPBKDF2, iterations: 600_000}, []byte{129, 57, 137, 140, 156, 220, 110, 212, 201, 255, 52, 182, 22, 206, 221, 66, 136, 199, 181, 89, 252, 175, 82, 168, 79, 204, 88, 174, 166, 60, 52, 79}},
		{"Argon2id", derivation{function: kdfArgon2id, iterations: 3, memoryMiB: 64, parallelism: 4}, []byte{221, 57, 158, 206, 27, 154, 188, 170, 33, 198, 250, 144, 191, 231, 29, 74, 201, 102, 253, 77, 8, 128, 173, 111, 217, 41, 125, 9, 156, 52, 112, 140}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !test.scheme.supported() {
				t.Fatal("the vector's parameters are refused")
			}
			material, err := test.scheme.derive("test_password", []byte("test_email@example.com"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(material, test.want) {
				t.Fatalf("derived %v, want %v", material, test.want)
			}
		})
	}
}

func TestStretchExpandsWithoutAnExtractStep(t *testing.T) {
	material := bytes.Repeat([]byte{0x5a}, keyLength)
	stretched, err := stretch(material)
	if err != nil {
		t.Fatal(err)
	}
	// One block of HKDF-Expand is HMAC(PRK, info || 0x01), with the material as the PRK.
	for _, want := range []struct {
		info string
		key  []byte
	}{{"enc", stretched.encryption}, {"mac", stretched.authentication}} {
		block := hmac.New(sha256.New, material)
		block.Write(append([]byte(want.info), 1))
		if !bytes.Equal(block.Sum(nil), want.key) {
			t.Fatalf("the %s key is not HKDF-Expand of the material", want.info)
		}
	}
}

func TestEncStringMatchesTheBitwardenVector(t *testing.T) {
	key := []byte{81, 142, 1, 228, 222, 3, 3, 133, 34, 176, 35, 66, 150, 6, 109, 70, 190, 149, 47, 47, 89, 23, 144, 87, 92, 46, 220, 13, 148, 106, 162, 234, 202, 139, 136, 33, 16, 200, 8, 73, 176, 172, 185, 187, 224, 10, 65, 223, 228, 54, 92, 181, 8, 213, 162, 221, 117, 254, 245, 111, 55, 211, 77, 29}
	k := keys{encryption: key[:keyLength], authentication: key[keyLength:]}
	envelope, err := parseEncString("2.Dh7AFLXR+LXcxUaO5cRjpg==|uXyhubjAoNH8lTdy/zgJDQ==|cHEMboj0MYsU5yDRQ1rLCgxcjNbKRc1PWKuv8bpU5pM=")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := envelope.open(k)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "test" {
		t.Fatalf("decrypted %q", plaintext)
	}
	for name, part := range map[string][]byte{"iv": envelope.iv, "data": envelope.data, "mac": envelope.mac} {
		part[0] ^= 1
		if _, err := envelope.open(k); !errors.Is(err, errMACMismatch) {
			t.Fatalf("a tampered %s opened: %v", name, err)
		}
		part[0] ^= 1
	}
}

func TestParseEncStringRefusesOtherEnvelopes(t *testing.T) {
	iv := base64.StdEncoding.EncodeToString(make([]byte, aes.BlockSize))
	block := base64.StdEncoding.EncodeToString(make([]byte, aes.BlockSize))
	mac := base64.StdEncoding.EncodeToString(make([]byte, sha256.Size))
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{"type 0", "0." + iv + "|" + block, importers.ErrUnsupportedEncryption},
		{"type 1", "1." + iv + "|" + block + "|" + mac, importers.ErrUnsupportedEncryption},
		{"no type", iv + "|" + block + "|" + mac, importers.ErrUnsupportedEncryption},
		{"empty", "", importers.ErrUnsupportedEncryption},
		{"two parts", "2." + iv + "|" + block, importers.ErrUnrecognized},
		{"not Base64", "2." + iv + "|!!!|" + mac, importers.ErrUnrecognized},
		{"short iv", "2." + base64.StdEncoding.EncodeToString(make([]byte, 8)) + "|" + block + "|" + mac, importers.ErrUnrecognized},
		{"partial block", "2." + iv + "|" + base64.StdEncoding.EncodeToString(make([]byte, 20)) + "|" + mac, importers.ErrUnrecognized},
		{"no data", "2." + iv + "||" + mac, importers.ErrUnrecognized},
		{"short mac", "2." + iv + "|" + block + "|" + iv, importers.ErrUnrecognized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseEncString(test.value); !errors.Is(err, test.want) {
				t.Fatalf("parseEncString = %v, want %v", err, test.want)
			}
		})
	}
}

func TestEncStringRefusesMalformedPaddingAfterItsMAC(t *testing.T) {
	k := keysFor(t, "password", fastPBKDF2, exportSalt)
	tests := map[string][]byte{
		"zero":         append(bytes.Repeat([]byte{'a'}, 15), 0),
		"over a block": append(bytes.Repeat([]byte{'a'}, 15), aes.BlockSize+1),
		"uneven":       append(bytes.Repeat([]byte{'a'}, 13), 1, 3, 3),
	}
	for name, blocks := range tests {
		t.Run(name, func(t *testing.T) {
			envelope, err := parseEncString(sealBlocks(t, k, blocks))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := envelope.open(k); !errors.Is(err, errPadding) {
				t.Fatalf("open = %v", err)
			}
		})
	}
}

func TestDerivationKeepsToBitwardensRanges(t *testing.T) {
	argon2id := func(iterations, memoryMiB, parallelism int) derivation {
		return derivation{function: kdfArgon2id, iterations: iterations, memoryMiB: memoryMiB, parallelism: parallelism}
	}
	tests := []struct {
		name   string
		scheme derivation
		want   bool
	}{
		{"PBKDF2 at the least", derivation{function: kdfPBKDF2, iterations: 5_000}, true},
		{"PBKDF2 at the most", derivation{function: kdfPBKDF2, iterations: 2_000_000}, true},
		{"PBKDF2 below", derivation{function: kdfPBKDF2, iterations: 4_999}, false},
		{"PBKDF2 above", derivation{function: kdfPBKDF2, iterations: 2_000_001}, false},
		{"Argon2id at the least", argon2id(2, 16, 1), true},
		{"Argon2id at the most", argon2id(10, 1024, 16), true},
		{"Argon2id iterations below", argon2id(1, 64, 4), false},
		{"Argon2id iterations above", argon2id(11, 64, 4), false},
		{"Argon2id memory below", argon2id(3, 15, 4), false},
		{"Argon2id memory above", argon2id(3, 1025, 4), false},
		{"Argon2id parallelism below", argon2id(3, 64, 0), false},
		{"Argon2id parallelism above", argon2id(3, 64, 17), false},
		{"another function", derivation{function: 2, iterations: 600_000}, false},
	}
	for _, test := range tests {
		if got := test.scheme.supported(); got != test.want {
			t.Errorf("%s: supported = %v, want %v", test.name, got, test.want)
		}
	}
}
