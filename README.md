# AirPlay Server

At the moment, you can start the RAOP advertisment, and start the server, and then connect from iTunes/iOS to play music. Multiple connections (or even disconnect and reconnect) are not handled.

To advertise the RAOP service,

    $ clang reg.c && ./a.out

Pressing enter will deregister the service and stop the program. To launch the server
listening for connections,

    $ go run serve.go

## A little bit of history

### AirTunes

AirTunes came out as a strictly audio-only protocol, which allowed computers running iTunes, and later wi-fi enabled
iOS devices to stream audio to any AirTunes server. This service is advertised via the `_raop._tcp` subdomain in
zeroconf, along with a TXT record detailing the server's audio capabilities, along with whether the server is
password-protected or not. A client connecting to a server includes an "Apple-Challenge" header, a random (?) 18-byte
token, which the server has to combine with its MAC and IP address and then encrypt using an RSA private key, and
send back as an "Apple-Response" header. Once this challenge has been met, the client gives the server an RSA-encrypted
AES key, which the audio packet payloads are encrypted with for the rest of the connection. The server uses its private
key to decrypt the AES key, and then the rest is history (assuming you know how RTP works).

In 2004, [Jon Lech Johansen](http://en.wikipedia.org/wiki/Jon_Lech_Johansen) (writer of DeCSS) published a client
for AirTunes, [JustePort](http://nanocr.eu/software/justeport/), using an RSA public key which (I think) he found in
the iTunes binary. This allowed anybody to stream music to an AirTunes server, but did not allow people to become an
AirTunes server, since the private key was still missing.

In 2011, James Laird published [shairport], an AirTunes server, using the RSA private key he had extracted from an
AirPort Express. Now both the private and public keys are available for people to implement their own AirTunes services
(as I am doing).

### AirPlay

Everything so far has been strictly about the RAOP service. In 2010, Apple announced AirPlay, which would allow people
to stream movies and photos in addition to audio, to AirPlay devices (such as the Apple TV). AirPlay comes in as a second
service alongside the RAOP service, advertised via the `_airplay._tcp` subdomain in zeroconf. Displaying photos,
slideshows, and videos across this is rather open (there is no encryption of any sort, or challenge-response headers),
and well-documented at [nto].

Advertising the `_airplay._tcp` in addition to `_raop._tcp` causes iOS clients to authenticate differently. I need to
explore more about this.

### AirPlay Mirroring

In 2011, Intel's Sandy Bridge (and onwards) processors enabled real-time video encoding and decoding, and AirPlay
Mirroring was released, which allowed Macs (from 2011 onwards) to mirror their displays to an Apple TV. iOS devices are
also able to mirror their displays to an Apple TV. The service is advertised on the `_airplay._tcp` TXT record, by
setting relevant bits in the features bitfield.

At the moment, there is no known open-source implementation of this protocol. However, there are commercial applications
which can do this, such as [AirServer](http://www.airserver.com/), which acts as a server,
[AirParrot](http://www.airsquirrels.com/airparrot/), which acts as a client, and [rplay](http://rplay.doit.org/), another
server, aimed at embedded devices (Raspberry Pi).


## RAOP TXT Record parameters

Most stuff here is duplicated from [nto], I'm just listing them to separate the
audio configuration stuff from the authentication/encryption stuff. This will hopefully
end up all documented in the code.

Non-authentication parameters:
* `txtvers=1` Always 1.
* `ch=2` Audio channels.
* `cn=0,1,2,3` Available codecs (PCM, ALAC, AAC, AAC ELD).
* `sr=44100` Audio sample rate.
* `ss=16` Audio sample size.
* `tp=UDP` Transport (Investigate... what does TCP do?)

Authentication parameters:
* `et=0,1,3,4,5` Supported encryption types. (0: None, 1: RSA (Airport Express),
  3: FairPlay, 4: MFiSAP, 5: FairPlay SAPv2.5)
* `md=0,1,2` Metadata type. It seems that if this is specified, then clients will send over data. (0: text, 1: artwork, 2: progress)
* `pw=false` Do we require a password?
* `vs=130.14` Server version (definitely makes the iPhone consider things differently)
* `am=AppleTV2,1` Device Model, (Speaker reports as `AirPort4,107`)

Unknown:
* `ek=1` ??? (Perhaps indicates that encryption will be used, "Encryption Key")
* `vn=3` ??? (Sub-version number? Try changing down and seeing headers?)

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
create the Apple-Response, see [shairport], steps outlined below:

* Allocate a buffer.
* Write in the decoded Apple-Challenge bytes.
* Write in the 16-byte IPv6 or 4-byte IPv4 address (network byte order).
* Write in the 6-byte Hardware address of the network interface (See note).
* If the buffer has less than 32 bytes written, pad with 0's up to 32 bytes.
* Encrypt the buffer using the RSA private key extracted in [shairport].
* Base64 encode the ciphertext, trim trailing '=' signs, and send back.

Note: iTunes seems to think your hardware address is what you say it is when you
publish the `_raop._tcp` name.

## RTSP Session

After restarting iTunes, a typical Airtunes RTSP session looks like this:

* User (not playing music) clicks on AirTunes device.
* iTunes requests the `OPTIONS` method, loaded with an `Apple-Challenge`
  header. If this is answered incorrectly, iTunes will display an error message
  to the user and not enable AirTunes. If this is answered correctly, the AirTunes
  blue active symbol is active. In  both cases, iTunes closes the TCP connection afterwards.
* The user starts to play some music. An `ANNOUNCE` is sent, about the audio and encryption
  parameters. Then a `SETUP` call to arrange ports. A `RECORD` call then signals that the audio
  session is starting. After that, two `SET_PARAMETER` calls, for volume (the same volume).
* If the user does nothing now, an `OPTIONS` call is made every 15 seconds, without any
  Apple-Challenge header.
* If the user skips a song, a `FLUSH` call is made.

## TODO

* Structure program well, so that clients can connect, reconnect, disconnect
  without resources going wonky.
* Add some command line flags for different modes of "sniffing around" (print out headers,
  indicate stream being recieved without actually doing anything).
* Handle more audio codecs, investigate whether codecs can be forced, and on what media.
  The other audio parameters (channels, sample rate, sample size) can happen later. If we
  turn off encryption, will anything play to us?
* Figure out those mystery RAOP TXT records.
* Implement AirPlay stuff (how do we display video/photos?)
* Try to get mirroring working (this would be a total win, probably won't happen).


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

## References/Sources

* Fairly complete documentation of RTSP and AirPlay protocol: [nto]
* A little extra documentation: [airtunes2]
* AirTunes server implementation, including private key: [shairport]
* Alac decoder implementation: [alac]


[nto]: http://nto.github.io/AirPlay.html
[shairport]: https://github.com/abrasive/shairport
[airtunes2]: http://git.zx2c4.com/Airtunes2/about/
[alac]: http://crazney.net/programs/itunes/alac.html
