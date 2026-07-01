Sound files in this folder are generated as standard RIFF PCM S16LE stereo 44100 Hz WAV
for maximum compatibility (desktop players, Web Audio, HTMLAudio).

Previously bundled Mixkit URLs sometimes served WAVEFORMATEXTENSIBLE 24-bit PCM; several
players decode those as silence.

Regenerate:
  python generate_sfx.py
