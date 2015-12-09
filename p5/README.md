Key items to look at:

crypto.go contains all funcations for encrypting / decrypting messages.
Specifically this happens in prepare_* and accept_*. These functions are
called prior to sending any message and after receiving a message.

Each RPC function in rpc.go now prepares / accepts requests / responses and
anytime one of these function is called the arguments are encrypted.

Authentication into the system happens in main.go