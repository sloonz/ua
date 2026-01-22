# ua-send

`ua-send` is a tool to send messages in the same JSON format as
`maildir-put` to a SMTP or LMTP server.

## Usage

	message-producer | ua-filter [arguments] | ua-send [arguments]

Available arguments:

* `-server`: SMTP/LMTP server address. For TCP use `host:port`. For LMTP
  over unix sockets, pass the socket path and `-lmtp-network unix`.
* `-lmtp`: use LMTP instead of SMTP.
* `-lmtp-network`: network for LMTP (`tcp` or `unix`).
* `-starttls`: enable LMTP STARTTLS (tcp only).
* `-insecure`: skip TLS certificate verification (STARTTLS).
* `-user`, `-password`: SMTP auth settings (PLAIN). Not supported for LMTP.
* `-from`: override sender address (envelope and From header).
* `-default-from`: sender address to use when missing (defaults to `-to`).
* `-to`: recipient list (envelope and To header).
* `-folder`: folder name to send in a header for server-side filing.
* `-folder-header`: header name to use with `-folder`. Defaults to
  `x-fileinto`.

## Installation

	go build && cp ua-send /usr/local/bin

## Input format

`ua-send` takes the same stream of JSON dictionaries as `maildir-put`.
All strings must be encoded in UTF-8.
