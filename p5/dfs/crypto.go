package dfs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

const (
	KEY_PREFIX = "keys/"
	PRI_SUFFIX = "_private.key"
	PUB_SUFFIX = "_public.key"
)

var (
	md5hash hash.Hash
	AESkey  []byte
)

//=====================================================================
func pubToString(pub *rsa.PublicKey) string {
	bytes, err := json.Marshal(pub)
	if err != nil {
		p_err("Public key json marshal err\n")
		return ""
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func sha256bytesToString(buf []byte) string {
	sha := sha256.Sum256(buf)
	shasl := sha[:]
	return base64.StdEncoding.EncodeToString(shasl)
}

//=====================================================================
// Create RSA keypair, save to a local file, also write a .pem file.
func mkKeyPair(name string) error {
	md5hash = md5.New()

	// generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	p_dieif(err != nil, "private key create: %q", err)
	publicKey := &privateKey.PublicKey

	bytes2 := RSAEncrypt(publicKey, []byte("Keys verified"))
	bytes3 := RSADecrypt(privateKey, bytes2)
	p_err("rsa result: %q\n", string(bytes3))

	// save keys
	writePublicKey(publicKey, name)
	writePrivateKey(privateKey, name)

	return nil
}

func RSAEncrypt(pub *rsa.PublicKey, plain []byte) []byte {
	md5hash = md5.New()
	cipher, err := rsa.EncryptOAEP(md5hash, rand.Reader, pub, plain, []byte(""))
	p_dieif(err != nil, "RSA encrypt error: %q\n", string(plain))
	return cipher
}

func RSADecrypt(privateKey *rsa.PrivateKey, cipher []byte) []byte {
	md5hash = md5.New()
	plain, err := rsa.DecryptOAEP(md5hash, rand.Reader, privateKey, cipher, []byte(""))
	p_dieif(err != nil, "RSA decrypt error: %v\n", cipher)
	return plain
}

//=====================================================================
func ReadPublicKey(fname string) *rsa.PublicKey {
	fname = KEY_PREFIX + fname + PUB_SUFFIX
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		p_err("ERROR reading public key: %q\n", fname)
		return nil
	}

	key := new(rsa.PublicKey)
	bytes2, err := base64.StdEncoding.DecodeString(string(bytes))
	p_dieif(err != nil, "base64 public decoding error\n")
	err = json.Unmarshal(bytes2, &key)
	p_dieif(err != nil, "json public decoding error\n")
	return key
}

func writePublicKey(key *rsa.PublicKey, fname string) error {
	bytes, err := json.Marshal(key)
	if err != nil {
		p_err("Public key json marshal err %q\n", fname)
		return err
	}
	str := base64.StdEncoding.EncodeToString(bytes)
	err = ioutil.WriteFile(KEY_PREFIX+fname+PUB_SUFFIX, []byte(str), 0644)
	p_err("wrote %q\n", KEY_PREFIX+fname+PUB_SUFFIX)
	return err
}

func ReadPrivateKey(fname string) *rsa.PrivateKey {
	fname = KEY_PREFIX + fname + PRI_SUFFIX
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		p_err("ERROR reading private key: %q\n", fname)
		return nil
	}

	key := new(rsa.PrivateKey)
	bytes2, err := base64.StdEncoding.DecodeString(string(bytes))
	p_dieif(err != nil, "base64 private decoding error\n")
	err = json.Unmarshal(bytes2, &key)
	p_dieif(err != nil, "json private decoding error\n")
	return key
}

func writePrivateKey(key *rsa.PrivateKey, fname string) error {
	bytes, err := json.Marshal(key)
	if err != nil {
		p_err("Private key json marshal err %q\n", fname)
		return err
	}
	str := base64.StdEncoding.EncodeToString(bytes)
	err = ioutil.WriteFile(KEY_PREFIX+fname+PRI_SUFFIX, []byte(str), 0644)
	p_err("wrote %q\n", KEY_PREFIX+fname+PRI_SUFFIX)
	return err
}

//=====================================================================

// Demo code: takes a file name and creates .encrypted and .decrypted
// versions.
func encryptStuff(args []string) {
	inName := args[0]
	outName := args[1]

	data, err := ioutil.ReadFile(inName)
	p_dieif(err != nil, "Problem reading %q\n", inName)

	key := make([]byte, aes.BlockSize)
	_, err = rand.Read(key[:])
	p_dieif(err != nil, "new AES key: %q\n", err)

	enc := aesEncrypt(key, data)
	ioutil.WriteFile(outName+".encrypted", enc, 0644)

	dec := AESDecrypt(key, enc)
	ioutil.WriteFile(outName+".decrypted", dec, 0644)

	//	p_err("Verify that %q == %q\n", inName, outName+".decrypted")
}

//
//  Encryption of a file w/ AES in CTR mode, use CTR
//  so that we don't have to pad.
//
func aesEncrypt(key, plaintext []byte) []byte {
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	_, err := io.ReadFull(rand.Reader, iv)
	p_dieif(err != nil, "readfull %q\n", err)

	aesBlock, err := aes.NewCipher(key)
	p_dieif(err != nil, "new AES blockcipher: %q\n", err)
	encrypter := cipher.NewCTR(aesBlock, iv)

	encrypter.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext
}

func AESDecrypt(key, ciphertext []byte) []byte {
	aesBlock, err := aes.NewCipher(key)
	p_dieif(err != nil, "new AES blockcipher: %q\n", err)
	stream := cipher.NewCTR(aesBlock, ciphertext[:aes.BlockSize])

	stream.XORKeyStream(ciphertext[aes.BlockSize:], ciphertext[aes.BlockSize:])
	return ciphertext[aes.BlockSize:]
}

func prepare_response(ack bool, pid int, block []byte, dn *DNode) []byte {
	res := Response{ack, pid, block, dn, nil, nil}
	encrypted := aesEncrypt(AESkey, Marshal(res))

	mac := hmac.New(sha256.New, AESkey)
	mac.Write(encrypted)

	m := new(Message)
	m.Res = encrypted
	m.HMAC = mac.Sum(nil)

	p_out("prepare_response: %s\n", sha256bytesToString(m.HMAC))
	return aesEncrypt(AESkey, Marshal(m))
}

func prepare_request(sig string, pid int) []byte {
	req := Request{sig, pid, 0}
	encrypted := aesEncrypt(AESkey, Marshal(req))

	mac := hmac.New(sha256.New, AESkey)
	mac.Write(encrypted)

	m := new(Message)
	m.Req = encrypted
	m.HMAC = mac.Sum(nil)

	p_out("prepare_request: %s\n", sha256bytesToString(m.HMAC))
	return aesEncrypt(AESkey, Marshal(m))
}

func accept_response(encrypted []byte) *Response {
	p_out("accept_response: %s\n", sha256bytesToString(encrypted))
	decrypted_m := AESDecrypt(AESkey, encrypted)
	p_out("accept_response: %s\n", decrypted_m)

	m := new(Message)
	_ = json.Unmarshal(decrypted_m, &m)
	p_dieif(!CheckMAC(m.Res, m.HMAC, AESkey), "HMAC FAIL!\n")

	res := new(Response)
	decrypted_r := AESDecrypt(AESkey, m.Res)
	_ = json.Unmarshal(decrypted_r, &res)
	return res
}

func accept_request(encrypted []byte) *Request {
	p_out("accept_request: %s\n", sha256bytesToString(encrypted))
	decrypted := AESDecrypt(AESkey, encrypted)

	m := new(Message)
	_ = json.Unmarshal(decrypted, &m)
	p_dieif(!CheckMAC(m.Req, m.HMAC, AESkey), "HMAC FAIL!\n")

	req := new(Request)
	decrypted_r := AESDecrypt(AESkey, m.Req)
	_ = json.Unmarshal(decrypted_r, &req)
	return req
}

func CheckMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

//=====================================================================
func usage() {
	x := os.Args[0][:]
	if i := strings.LastIndexByte(x, '/'); i >= 0 {
		x = x[i+1:]
	}
	p_err("USAGE: %s"+`
            newkeypair <base file name>
            aes <source> <dest basename>
           `, x)
	os.Exit(2)
}

// func main() {
// 	var err error
// 	args := os.Args
// 	num := len(args)
// 	if num < 2 {
// 		usage()
// 	}
// 	switch args[1] {
// 	case "newkeypair":
// 		if num != 3 {
// 			usage()
// 		}
// 		err = mkKeyPair(args[2])
// 	case "aes":
// 		if num != 4 {
// 			usage()
// 		}
// 		encryptStuff(args[2:])
// 	}
// 	if err != nil {
// 		p_dieif(true, "FAIL: %q\n\t%s\n", strings.Join(args, " "))
// 	}
// }
