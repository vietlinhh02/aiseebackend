package assistant

import "errors"

var errMissingMediaMode = errors.New("media_mode payload.mode is required")
var errInvalidMediaMode = errors.New("media_mode payload.mode must be audio_pcm or camera_jpeg")
