#include "_cgo_export.h"

int32_t setup_queue(
	ALACMagicCookie cookie,
	PlayerInfo *playerInfo,
	uint32_t buffer_size,
	uint32_t num_buffers,
	uint32_t num_packets
) {
  // Create Audio Queue for ALAC
  AudioStreamBasicDescription inFormat = {0};
  inFormat.mSampleRate = ntohl(cookie.sampleRate);
  inFormat.mFormatID = kAudioFormatAppleLossless;
  inFormat.mFormatFlags = 0; // ALAC uses no flags
  inFormat.mBytesPerPacket = 0; // Variable size (must use AudioStreamPacketDescription)
  inFormat.mFramesPerPacket = ntohl(cookie.frameLength);
  inFormat.mBytesPerFrame = 0; // Compressed
  inFormat.mChannelsPerFrame = 2; // Stero TODO: get from fmtp?
  inFormat.mBitsPerChannel = 0; // Compressed
  inFormat.mReserved = 0;
  
  OSStatus err = AudioQueueNewOutput(
      &inFormat,
      c_callback,
      playerInfo, // User data
      NULL, // Run on audio queue's thread
      NULL, // Callback run loop's mode
      0, // Reserved
      &playerInfo->queue);

  if (err) return err;

  // Need to set the magic cookie too (tail fmtp)
  err = AudioQueueSetProperty(playerInfo->queue, kAudioQueueProperty_MagicCookie,
			&cookie, sizeof(ALACMagicCookie));
  if (err) return err;

	// Create input buffers, and enqueue using callback
	for (int i = 0; i < num_buffers; i++) {
		AudioQueueBufferRef buffer;
		err = AudioQueueAllocateBufferWithPacketDescriptions(
				playerInfo->queue, buffer_size, num_packets, &buffer);
		if (err) return err;
		
		c_callback(playerInfo, playerInfo->queue, buffer);
	}

	// Volume full
	err = AudioQueueSetParameter(playerInfo->queue, kAudioQueueParam_Volume, 1.0);
	if (err) return err;

  // Prime
  err = AudioQueuePrime(playerInfo->queue, 0, NULL);
  if (err) return err;

	// Start
	err = AudioQueueStart(playerInfo->queue, NULL);
	if (err) return err;

	return 0;
}

void c_callback(
  void *user_data,
  AudioQueueRef queue,
  AudioQueueBufferRef buffer
) {
  Go_callback(user_data, queue, buffer);
}
