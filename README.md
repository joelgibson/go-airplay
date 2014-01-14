# AirPlay Server

At the moment, iTunes will connect (but not be able to play) to the server. I still don't
have iOS ready to connect - something is happening with the Apple-Challenge Apple-Response
there...

To advertise the RAOP service,

    $ clang reg.c && ./a.out

Pressing enter will deregister the service and stop the program. To launch the server
listening for connections,

    $ go run serve.go



## RAOP TXT Record parameters

Most stuff here is duplicated from [1], I'm just listing them to separate the
audio configuration stuff from the authentication/encryption stuff.

Non-authentication parameters:
* `txtvers=1` Always 1.
* `ch=2` Audio channels.
* `cn=0,1,2,3` Available codecs (PCM, ALAC, AAC, AAC ELD).
* `sr=44100` Audio sample rate.
* `ss=16` Audio sample size.
* `tp=UDP` Transport (Investigate...)

Authentication parameters:
* `et=0,1,3,4,5` Supported encryption types. (0: None, 1: RSA (Airport Express),
  3: FairPlay, 4: MFiSAP, 5: FairPlay SAPv2.5)
* `md=0,1,2` Metadata type (not sure if this makes the iPhone want to connect
  via Fairplay). (0: text, 1: artwork, 2: progress)
* `pw=false` Do we require a password?
* `vs=130.14` Server version (definitely makes the iPhone consider things differently)
* `am=AppleTV2,1` Device Model, (Speaker reports as `AirPort4,107`)

Unknown:
* `ek=1` ???
* `vn=3` ???

Testing: Using the following set of basic headers:

    txtvers=1
    pw=false
    tp=UDP
    sm=false
    ek=1
    cn=0,1
    ch=2
    ss=16
    sr=44100
    vn=3

The RAOP service only was advertised. If the encryption record `et=` was omitted, or had
value `et=0` (no encryption), the iPhone/iPad did not see it, but iTunes saw it as a
speaker. When iTunes did the OPTIONS call, it did not come with an Apple-Challenge header.

If we enabled encryption to none or RSA, `et=0,1`, then both iTunes and the iOS
devices saw it. An Apple-Challenge was seen from both iTunes and iOS (iTunes needed
a restart - once it challenges once sucessfully, it does not challenge the server again)

## Generating the Apple-Response from the Apple-Challenge

When RSA encryption is enabled, the first call made the the server looks like

    OPTIONS * RTSP/1.0
    CSeq: 1
    User-Agent: iTunes/11.1.3 (Macintosh; OS X 10.7.5) AppleWebKit/534.57.7
    Client-Instance: 44DF9EAA3D40A814
    DACP-ID: 44DF9EAA3D40A814
    Active-Remote: 2861677811
    Apple-Challenge: ZXB6HAnyFyuzScHTuf1aqQ

There is no request body. The Apple-Challenge field is a base64 encoded 16 byte
(there may be < 16 bytes sometimes?)
sequence (iTunes appears to omit the padding on the end, while iOS does not). An
Apple-Response header has to be sent back along with the OPTIONS response. To
create the Apple-Response, see [2], steps outlined below:

   * Allocate a buffer.
   * Write in the decoded Apple-Challenge bytes.
   * Write in the 16-byte IPv6 or 4-byte IPv4 address (network byte order).
   * Write in the 6-byte Hardware address of the network interface (See note).
   * Either trim or pad with 0's to 32 bytes long.
   * Encrypt the 32 bytes using the RSA private key extracted in [2].
   * Base64 encode the ciphertext, trim trailing '=' signs, and send back.

Note: iTunes seems to think your hardware address is what you say it is when you
publish the `_raop._tcp` name.




## Fairplay (blocked)
The message looks like this:

    POST /fp-setup RTSP/1.0
    X-Apple-ET: 32
    CSeq: 0
    X-Apple-Device-ID: 0x7cc537cc81ee
    DACP-ID: F906150062FC585D
    Active-Remote: 3989558941
    Content-Type: application/octet-stream
    Content-Length: 16
    User-Agent: AirPlay/190.9

The body seems to be a 16-byte messages, starting `FPLY`. I think this is a
challenge-response thing. No-one on the internet (in a solid hour or two of
googling) seems to have cracked it.


[1]: http://nto.github.io/AirPlay.html
[2]: https://github.com/abrasive/shairport/blob/master/rtsp.c
