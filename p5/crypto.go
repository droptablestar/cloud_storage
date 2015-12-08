package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

const (
	PRI_SUFFIX = "_private.key"
	PUB_SUFFIX = "_public.key"
)

var (
	md5hash hash.Hash
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

	bytes2 := rsaEncrypt(publicKey, []byte("Keys verified"))
	bytes3 := rsaDecrypt(privateKey, bytes2)
	p_err("rsa result: %q\n", string(bytes3))

	// save keys
	writePublicKey(publicKey, name)
	writePrivateKey(privateKey, name)

	return nil
}

func rsaEncrypt(pub *rsa.PublicKey, plain []byte) []byte {
	cipher, err := rsa.EncryptOAEP(md5hash, rand.Reader, pub, plain, []byte(""))
	p_dieif(err != nil, "RSA encrypt error: %q\n", string(plain))
	return cipher
}

func rsaDecrypt(privateKey *rsa.PrivateKey, cipher []byte) []byte {
	plain, err := rsa.DecryptOAEP(md5hash, rand.Reader, privateKey, cipher, []byte(""))
	p_dieif(err != nil, "RSA decrypt error: %v\n", cipher)
	return plain
}

//=====================================================================
func readPublicKey(fname string) *rsa.PublicKey {
	fname = fname + PUB_SUFFIX
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
	err = ioutil.WriteFile(fname+PUB_SUFFIX, []byte(str), 0644)
	p_err("wrote %q\n", fname+PUB_SUFFIX)
	return err
}

func readPrivateKey(fname string) *rsa.PrivateKey {
	fname = fname + PRI_SUFFIX
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
	err = ioutil.WriteFile(fname+PRI_SUFFIX, []byte(str), 0644)
	p_err("wrote %q\n", fname+PRI_SUFFIX)
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

	dec := aesDecrypt(key, enc)
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

func aesDecrypt(key, ciphertext []byte) []byte {
	aesBlock, err := aes.NewCipher(key)
	p_dieif(err != nil, "new AES blockcipher: %q\n", err)
	stream := cipher.NewCTR(aesBlock, ciphertext[:aes.BlockSize])

	stream.XORKeyStream(ciphertext[aes.BlockSize:], ciphertext[aes.BlockSize:])
	return ciphertext[aes.BlockSize:]
}

//=====================================================================
func p_err(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

func p_dieif(b bool, s string, args ...interface{}) {
	if b {
		p_err(s, args...)
		os.Exit(1)
	}
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

func main() {
	var err error
	args := os.Args
	num := len(args)
	if num < 2 {
		usage()
	}
	switch args[1] {
	case "newkeypair":
		if num != 3 {
			usage()
		}
		err = mkKeyPair(args[2])
	case "aes":
		if num != 4 {
			usage()
		}
		encryptStuff(args[2:])
	}
	if err != nil {
		p_dieif(true, "FAIL: %q\n\t%s\n", strings.Join(args, " "))
	}
}
