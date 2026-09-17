const assert = require('node:assert/strict');
const { synchronizePosition } = require('./playback-clock.js');

function synchronize(overrides = {}) {
  return synchronizePosition({
    hasCurrentTrack: true,
    trackChanged: false,
    wasPlaying: true,
    playing: true,
    estimatedPositionMs: 31_050,
    reportedPositionMs: 30_000,
    previousReportedPositionMs: 27_000,
    ...overrides,
  });
}

assert.equal(
  synchronize().positionMs,
  31_050,
  'a stale Plex sample must not move normal playback backward across a displayed second',
);

assert.equal(
  synchronize({ estimatedPositionMs: 30_000, reportedPositionMs: 31_000 }).positionMs,
  30_250,
  'a slightly advanced Plex sample should correct drift gradually',
);

const backwardSeek = synchronize({
  estimatedPositionMs: 33_000,
  reportedPositionMs: 20_000,
  previousReportedPositionMs: 30_000,
});
assert.equal(backwardSeek.positionMs, 20_000);
assert.equal(backwardSeek.backwardSeek, true, 'a confirmed rewind must reset immediately');

const forwardSeek = synchronize({
  estimatedPositionMs: 33_000,
  reportedPositionMs: 60_000,
  previousReportedPositionMs: 30_000,
});
assert.equal(forwardSeek.positionMs, 60_000);
assert.equal(forwardSeek.forwardSeek, true, 'a confirmed forward seek must reset immediately');

assert.equal(
  synchronize({ trackChanged: true, reportedPositionMs: 500 }).positionMs,
  500,
  'a new track must reset to its reported position',
);

assert.equal(
  synchronize({ wasPlaying: true, playing: false, reportedPositionMs: 30_000 }).positionMs,
  30_000,
  'pausing must use the reported position',
);

console.log('playback clock tests passed');
