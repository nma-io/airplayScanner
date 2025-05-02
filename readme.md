# AirPlay Scanner

Author: Nicholas Albright (@nma-io)

## Overview

Apple AirPlay runs on TCP port 7000 for any device that supports it, including MacBook, iPad?, iPhone?, AppleTV, and a host of third-party products. The information available for a device can be pulled via an HTTP GET request, without authentication, on port 7000 by hitting the endpoint `/info`.

A raw dump of this output is available in `output.bin`.

## Reverse Engineering

It appears that important fields are delimited by the hex `5f 10` followed by the size of the next string. The response is a binary property list (bplist00).

## Interesting Fields

- deviceID: Unique identifier (e.g., MAC address)
- features: Bitfield indicating supported features (e.g., video, audio, screen mirroring)
- statusFlags: Device state (e.g., availability, pairing status)
- sourceVersion: AirPlay software version
- model: Device model (e.g., AppleTV5,3)
- protocolVersion: AirPlay protocol version
- macAddress: Device MAC address
- name: Human-readable device name

## Technology & Patterns Used

Binary PropertyList Parsing from [`howett.net/plist`](https://pkg.go.dev/howett.net/plist)


## Usage

```
airplayScanner [-out results.csv] [-no-color] 192.168.1.0/24
```
- `-out`: Output file (CSV, optional; default: prints to screen)
- `-no-color`: Disable color output (optional)
[ip]: positional argument, just put it at the end.

## Output Example

Console (colorized):

```
[+] Found: Master Bedroom (AppleTV5,3) 192.168.1.43 MAC: 9A:EF:03:66:56:CD Features: ...
```

CSV file:

```
name,model,ip,macAddress,deviceID,features,statusFlags,sourceVersion,protocolVersion
Master Bedroom,AppleTV5,3,192.168.1.43,9A:EF:03:66:56:CD,9BEF0366-56CD-4EF6-B543-F0890BD24BA7,...
```




