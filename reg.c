#include <stdio.h>
#include <string.h>
#include <dns_sd.h>

/* Find some stuff here: http://nto.github.io/AirPlay.html#servicediscovery
 * And some more stuff here: https://developer.apple.com/library/mac/documentation/Networking/Conceptual/dns_discovery_api/Articles/registering.html#//apple_ref/doc/uid/TP40002478-SW1
 */

void write_txt(char *list[], char *buf) {
  while (*list != NULL) {
    *buf++ = strlen(*list);
    strcpy(buf, *list);
    buf += strlen(*list++);
  }
}

int main(int argc, char *argv[]) {
  DNSServiceRef raopRef, airplayRef;

  // Hard code my mac address in lol
  const char *name = "0019E3D9312B@JoelSwag";
  char *txt_raop_list[] = {
    /*"txtvers=1",
    "ch=2",           // Stereo
    "cn=0,1",     // All of the codecs
    //"da=true",        // ???
    "et=0,3,5",       // Encryption types
    //"md=0,1,2",       // Metadata types
    "pw=false",       // Password reqd?
    //"sv=false",       // ???
    "sr=44100",       // Audio sample rate (Hz)
    "ss=16",          // Audio sample size (bits)
    "tp=UDP",         // Suppored transport
    "vn=3",       // ???
    //"vs=130.14",      // Server version
    //"am=AppleTV2,1",  // Device model
    //"am=AirPort4,107",
    //"sf=0x4",         // ??? 0x4 registers as TV, 0x1 registers as speaker
    //"sf=0x1",
    "sm=false",
    "ek=1",
    NULL*/
    "txtvers=1",
    //"md=0,1,2",       // Metadata types
    "pw=false",
    "tp=UDP",
    "sm=false",
    "ek=1",
    "cn=0,1",
    "ch=2",
    "ss=16",
    "sr=44100",  // Sample rate
    "vn=3",
    "et=0,1",
    NULL
  };
  char *txt_airplay_list[] = {
    "deviceid=00:19:E3:D9:31:2B",
    //"features=0x39f7",  // Features bitfield
    "features=0x7",
    //"pw=1", // Password protected
    "model=AppleTV2,1",
    //"srcvers=130.14", //Disable this to get rid of /fp-setup
    NULL
  };
  char txt_raop[1024];
  char txt_airplay[1024];
  write_txt(txt_raop_list, txt_raop);
  write_txt(txt_airplay_list, txt_airplay);
  
  DNSServiceErrorType err = DNSServiceRegister(
      &raopRef,   // Service ref
      0,        // flags DnsServiceFlags
      0,        // interface index uint32_t
      name,     // Name const char*
      "_raop._tcp", // Service type const char*
      NULL,     // Domain const char*
      NULL,     // Host const char*
      0x00c0,   // Port (network byte order) uint16_t (49152)
      strlen(txt_raop),        // TXT len uint16_t
      txt_raop, // TXT record const void*
      NULL,     // Callback DNSServiceRegisterReply
      NULL);    // Application context pointer void*
  if (err != kDNSServiceErr_NoError) {
    fprintf(stderr, "Could not register RAOP service, code %d\n", err);
    return 1;
  }
  fprintf(stderr, "Successfully registered RAOP service!\n");
  /*err = DNSServiceRegister(
      &airplayRef,   // Service ref
      0,        // flags DnsServiceFlags
      0,        // interface index uint32_t
      "JoelSwag",     // Name const char*
      "_airplay._tcp", // Service type const char*
      NULL,     // Domain const char*
      NULL,     // Host const char*
      0x581b,   // Port (network byte order) uint16_t (7000)
      strlen(txt_airplay),        // TXT len uint16_t
      txt_airplay, // TXT record const void*
      NULL,     // Callback DNSServiceRegisterReply
      NULL);    // Application context pointer void*
  if (err != kDNSServiceErr_NoError) {
    fprintf(stderr, "Could not register airplay service, code %d\n", err);
  }
  fprintf(stderr, "Successfully registered airplay service!\n");*/
  fprintf(stderr, "Waiting for you... ");
  getchar();

  fprintf(stderr, "Deregistering services.\n");
  DNSServiceRefDeallocate(raopRef);
  //DNSServiceRefDeallocate(airplayRef);

  return 0;
}
