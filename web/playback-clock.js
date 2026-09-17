(function initializePlaybackClock(globalScope) {
  const SEEK_THRESHOLD_MS = 1500;
  const DRIFT_CORRECTION_FACTOR = 0.25;

  function synchronizePosition({
    hasCurrentTrack,
    trackChanged,
    wasPlaying,
    playing,
    estimatedPositionMs,
    reportedPositionMs,
    previousReportedPositionMs,
  }) {
    const playbackChanged = wasPlaying !== playing;
    const backwardSeek = hasCurrentTrack
      && !trackChanged
      && wasPlaying
      && playing
      && previousReportedPositionMs !== null
      && reportedPositionMs < previousReportedPositionMs - SEEK_THRESHOLD_MS;
    const forwardSeek = hasCurrentTrack
      && !trackChanged
      && reportedPositionMs > estimatedPositionMs + SEEK_THRESHOLD_MS;
    const hardSync = !hasCurrentTrack || trackChanged || playbackChanged || backwardSeek || forwardSeek;

    if (hardSync || !playing) {
      return { positionMs: reportedPositionMs, backwardSeek, forwardSeek };
    }

    const correctedPosition = estimatedPositionMs
      + (reportedPositionMs - estimatedPositionMs) * DRIFT_CORRECTION_FACTOR;
    return {
      positionMs: Math.max(estimatedPositionMs, correctedPosition),
      backwardSeek,
      forwardSeek,
    };
  }

  const api = { synchronizePosition };
  if (typeof module === 'object' && module.exports) {
    module.exports = api;
  } else {
    globalScope.PlaybackClock = api;
  }
}(typeof globalThis === 'object' ? globalThis : this));
