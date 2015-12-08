#

# create public keypair
go run crypto.go newkeypair local1
go run crypto.go newkeypair local2
go run crypto.go newkeypair local3

go run crypto.go aes make.sh test
echo "Verifying: make.sh == test.decrypted"
cmp make.sh test.decrypted

