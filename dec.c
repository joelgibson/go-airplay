#include <stdio.h>
#include <stdlib.h>
#include <AudioToolbox/AudioToolbox.h>
//#include <CoreAudio/CoreAudioTypes.h>

#define NUMBUFS (8)
#define BUFSIZE (1024 * 8)

typedef unsigned char byte;

void fail(char *msg) {
  fprintf(stderr, "%s\n", msg);
  exit(1);
}
typedef struct ALACSpecificConfig {
  uint32_t  frameLength;
  uint8_t   compatibleVersion;
  uint8_t   bitDepth;
  uint8_t   pb;
  uint8_t   mb;
  uint8_t   kb;
  uint8_t   numChannels;
  uint16_t  maxRun;
  uint32_t  maxFrameBytes;
  uint32_t  avgBitRate;
  uint32_t  sampleRate;
} ALACSpecificConfig;

AudioQueueBufferRef bufs[NUMBUFS];

FILE *inFile;
AudioQueueRef outAQ;

// This callback gets handed a queue element (inBuffer). It's job is to fill it up with packets
// and send it off, or to stop playing if there are no more. Since ALAC has variable size
// packets, we need to store that info too.
void inCallbackProc(void *inUserData, AudioQueueRef inAQ, AudioQueueBufferRef inBuffer) {
  int bufpos = 0;
  int packets = 0;
  while (!feof(inFile)) {
    // Find the size of the next packet
    byte sizeb[2];
    fread(sizeb, 2, 1, inFile);
    int psize = (sizeb[0]<<8) + sizeb[1];
    if (bufpos + psize > BUFSIZE) {
      fseek(inFile, -2, SEEK_CUR);
      break;
    }
    fread(&inBuffer->mAudioData[bufpos], psize, 1, inFile);
    inBuffer->mPacketDescriptions[packets].mStartOffset = bufpos;
    inBuffer->mPacketDescriptions[packets].mVariableFramesInPacket = 0; // Not variable
    inBuffer->mPacketDescriptions[packets].mDataByteSize = psize;

    packets++;
    bufpos += psize;
  }
  if (bufpos == 0) {
    AudioQueueStop(outAQ, false);
    return;
  }

  inBuffer->mAudioDataByteSize = bufpos;
  inBuffer->mPacketDescriptionCount = packets;

  printf("Queueing a buffer of %d bytes containing %d packets\n", bufpos, packets);

  // Enqueue: the (0, NULL) means read the mPacketDescriptions member
  OSStatus err = AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL);
  if (err) {
    fprintf(stderr, "Could not enqueue: %d\n", err);
    fail((char*)&err);
  }
}

int main() {
  // This is from the ANNOUNCE call
	int fmtp[] = {96, 352, 0, 16, 40, 10, 14, 2, 255, 0, 0, 44100};

  // Create Audio Queue for ALAC
  AudioStreamBasicDescription inFormat = {0};
  inFormat.mSampleRate = fmtp[11];
  inFormat.mFormatID = kAudioFormatAppleLossless;
  inFormat.mFormatFlags = 0; // ALAC uses no flags
  inFormat.mBytesPerPacket = 0; // Variable size (must use AudioStreamPacketDescription)
  inFormat.mFramesPerPacket = fmtp[1];
  inFormat.mBytesPerFrame = 0; // Compressed
  inFormat.mChannelsPerFrame = 2; // Stero TODO: get from fmtp?
  inFormat.mBitsPerChannel = 0; // Compressed
  inFormat.mReserved = 0;
  
  OSStatus err = AudioQueueNewOutput(
      &inFormat,
      inCallbackProc,
      NULL, // User data
      NULL, // Run on audio queue's thread
      NULL, // Callback run loop's mode
      0, // Reserved
      &outAQ);
  if (err) {
    fprintf(stderr, "Could not create audio queue\n");
    fail((char *)&err);
  }

  // Need to set the magic cookie too (tail fmtp)
  ALACSpecificConfig cookie = {htonl(352), 0, 16, 40, 10, 14, 2, 255, 0, 0, htonl(44100)};
  err = AudioQueueSetProperty(
    outAQ,
    kAudioQueueProperty_MagicCookie,
    &cookie,
    sizeof(ALACSpecificConfig));
  if (err) {
    fprintf(stderr, "Could not set the maagic cookie\n");
    fail((char *)&err);
  }

  // Open the input file
  inFile = fopen("datafile", "r");
  if (inFile == NULL)
    fail("Could not open file");

  for (int i = 0; i < NUMBUFS; i++) {
    err = AudioQueueAllocateBufferWithPacketDescriptions(
        outAQ,
        BUFSIZE, // Size of audio buffer
        BUFSIZE, // Number of packet descriptions (FAR too many)
        &bufs[i]);
    if (err) {
      fprintf(stderr, "Could not allocate audio queue buffer\n");
      fail((char *)&err);
    }
    inCallbackProc(NULL, outAQ, bufs[i]);
  }

  err = AudioQueueSetParameter(outAQ, kAudioQueueParam_Volume, 1.0);
  if (err) {
    fprintf(stderr, "Error setting gain: %d\n", err);
    fail((char *)&err);
  }

  err = AudioQueuePrime(outAQ, 0, NULL);
  if (err) {
    fprintf(stderr, "Error priming: %d\n", err);
    fail((char *)&err);
  }
  
  err = AudioQueueStart(outAQ, NULL);
  if (err) {
    fprintf(stderr, "Could not start playing %d\n", err);
    fail((char *)&err);
  }
  fprintf(stderr, "Started\n");

  printf("Waiting..."); getchar();

  return 0;
}
