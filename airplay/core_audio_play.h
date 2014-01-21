// This is here for reference, it gets passed in from Go.
typedef struct ALACMagicCookie {
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
} ALACMagicCookie;

// This is a player instance, which C and Go share.
typedef struct PlayerInfo {
	AudioQueueRef queue;
} PlayerInfo;

// Sets up a Core Audio queue to play ALAC. Returns the Core Audio's
// error codes on fail (forwarded through).
int32_t setup_queue(
	ALACMagicCookie cookie,
	PlayerInfo *playerInfo,
	uint32_t buffer_size,
	uint32_t num_buffers, 
	uint32_t num_packets    // Max number of packets in a buffer
);

// Forwards the callback through to Go
void c_callback(
  void *user_data,
  AudioQueueRef queue,
  AudioQueueBufferRef buffer
);
