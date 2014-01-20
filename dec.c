#include <stdio.h>
#include <stdlib.h>
#include <AudioToolbox/AudioToolbox.h>
#include "alac.h"
#include <CoreAudio/CoreAudioTypes.h>

#define BUFSIZE (1024 * 32)

typedef unsigned char byte;

UInt32 CalculateLPCMFlags (
   UInt32 inValidBitsPerChannel,
   UInt32 inTotalBitsPerChannel,
   bool inIsFloat,
   bool inIsBigEndian,
   bool inIsNonInterleaved
) {
  inIsNonInterleaved = false;
   return
   (inIsFloat ? kAudioFormatFlagIsFloat : kAudioFormatFlagIsSignedInteger) |
   (inIsBigEndian ? ((UInt32)kAudioFormatFlagIsBigEndian) : 0)             |
   ((!inIsFloat && (inValidBitsPerChannel == inTotalBitsPerChannel)) ?
   kAudioFormatFlagIsPacked : kAudioFormatFlagIsAlignedHigh)           |
   (inIsNonInterleaved ? ((UInt32)kAudioFormatFlagIsNonInterleaved) : 0);
}

void FillOutASBDForLPCM (
   AudioStreamBasicDescription *outASBD,
   Float64 inSampleRate,
   UInt32 inChannelsPerFrame,
   UInt32 inValidBitsPerChannel,
   UInt32 inTotalBitsPerChannel,
   bool inIsFloat,
   bool inIsBigEndian,
   bool inIsNonInterleaved
) {
  inIsNonInterleaved = false;
   outASBD->mSampleRate = inSampleRate;
   outASBD->mFormatID = kAudioFormatLinearPCM;
   outASBD->mFormatFlags =    CalculateLPCMFlags (
   inValidBitsPerChannel,
   inTotalBitsPerChannel,
   inIsFloat,
   inIsBigEndian,
   inIsNonInterleaved
   );
   outASBD->mBytesPerPacket =
   (inIsNonInterleaved ? 1 : inChannelsPerFrame) * (inTotalBitsPerChannel/8);
   outASBD->mFramesPerPacket = 1;
   outASBD->mBytesPerFrame =
   (inIsNonInterleaved ? 1 : inChannelsPerFrame) * (inTotalBitsPerChannel/8);
   outASBD->mChannelsPerFrame = inChannelsPerFrame;
   outASBD->mBitsPerChannel = inValidBitsPerChannel;
}

int host_bigendian = 0;

void fail(char *msg) {
  fprintf(stderr, "%s\n", msg);
  exit(1);
}

const int numbufs = 10;
AudioQueueBufferRef bufs[numbufs];

byte *data = NULL;
int datapos = 0;
int datalen = 0;
AudioQueueRef outAQ;

void inCallbackProc(void *inUserData, AudioQueueRef inAQ, AudioQueueBufferRef inBuffer) {
  // This gets called when inBuffer has been acquired by the queue.
  if (datalen - datapos < BUFSIZE) {
    AudioQueueStop(outAQ, false);
    return;
  }

  printf("Queueing %d\n", datapos);
  memcpy(inBuffer->mAudioData, &data[datapos], BUFSIZE);
  inBuffer->mAudioDataByteSize = BUFSIZE;
  datapos += BUFSIZE;
  OSStatus err = AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL);
  if (err) {
    fprintf(stderr, "Could not enqueue: %d\n", err);
    fail((char*)&err);
  }
}

int main() {
  // Read the file into memory (20MB should be enough)
  byte *infile = malloc(sizeof(byte) * 1024*1024*20);
  FILE *fp = fopen("datafile", "r");
  if (fp == NULL)
    fail("Could not open file.");
  int pos = 0, c;
  while ((c = fgetc(fp)) != EOF)
    infile[pos++] = c;
  int filelen = pos;

	int fmtp[] = {96, 352, 0, 16, 40, 10, 14, 2, 255, 0, 0, 44100};
	int frame_size = fmtp[1];
	alac_file *alac;
	frame_size = fmtp[1]; // stereo samples
    int sampling_rate = fmtp[11];

    int sample_size = fmtp[3];
    if (sample_size != 16)
       fail("only 16-bit samples supported!");

    alac = alac_create(sample_size, 2);
    if (!alac)
			fail("Not alac");

    alac->setinfo_max_samples_per_frame = frame_size;
    alac->setinfo_7a =      fmtp[2];
    alac->setinfo_sample_size = sample_size;
    alac->setinfo_rice_historymult = fmtp[4];
    alac->setinfo_rice_initialhistory = fmtp[5];
    alac->setinfo_rice_kmodifier = fmtp[6];
    alac->setinfo_7f =      fmtp[7];
    alac->setinfo_80 =      fmtp[8];
    alac->setinfo_82 =      fmtp[9];
    alac->setinfo_86 =      fmtp[10];
    alac->setinfo_8a_rate = fmtp[11];
    alac_allocate_buffers(alac);

	// Destination buffer
	data = malloc(sizeof(byte) * 1024*1024*20);
	int destpos = 0;
	pos = 0;

  // Audio Queue
  AudioStreamBasicDescription inFormat = {0};
  FillOutASBDForLPCM(
      &inFormat,
      44100,
      2, // channels per frame
      16, // bits per channel
      16, // bits per channel
      false, // is float?
      false, // is big endian?
      false // is interleaved?
  );
  
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



	while (pos < filelen) {
		int framelen = (infile[pos]<<8) + infile[pos+1];
		pos += 2;
		int nwrote = 1024*1024*20;
		alac_decode_frame(alac, &infile[pos], &data[destpos], &nwrote);
    destpos += nwrote;
		pos += framelen;
		printf("In: %d, Out: %d\n", framelen, nwrote);
	}
  datalen = destpos;


	fp = fopen("out.pcm", "w");
	printf("ssize: %d, srate: %d\n", sample_size, sampling_rate);
	fwrite(data, datalen, 1, fp);
	fclose(fp);

  for (int i = 0; i < numbufs; i++) {
    err = AudioQueueAllocateBuffer(outAQ, BUFSIZE, &bufs[i]);
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
