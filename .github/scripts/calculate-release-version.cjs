const { execFileSync } = require("node:child_process");
const { appendFileSync, existsSync, readFileSync } = require("node:fs");

function parseVersion(value) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(value);
  return match ? { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) } : null;
}

function parseReleaseCandidate(value) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)-rc\.(\d+)$/.exec(value);
  return match ? { version: { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) }, number: Number(match[4]), tag: value } : null;
}

function compareVersions(left, right) {
  return left.major - right.major || left.minor - right.minor || left.patch - right.patch;
}

function formatVersion(version) {
  return `${version.major}.${version.minor}.${version.patch}`;
}

function findLatestStableTag(tags) {
  return tags.map((tag) => ({ tag, version: parseVersion(tag) }))
    .filter(({ version }) => version !== null)
    .sort((left, right) => compareVersions(right.version, left.version))[0] || null;
}

function requiredBump(messages) {
  let bump = "none";
  for (const message of messages) {
    const conventionalHeaders = message.split(/\r?\n/)
      .map((line) => /^([a-z][a-z0-9-]*)(?:\([^\r\n)]+\))?(!)?:/.exec(line.trim()))
      .filter(Boolean);
    const releaseHeaders = conventionalHeaders.filter((header) => ["feat", "fix", "perf", "revert"].includes(header[1]));
    if (releaseHeaders.length === 0) continue;
    if (releaseHeaders.some((header) => header[2] === "!") || /(^|\r?\n)BREAKING(?: CHANGE|-CHANGE):/m.test(message)) return "major";
    if (releaseHeaders.some((header) => header[1] === "feat")) bump = "minor";
    else if (bump === "none") bump = "patch";
  }
  return bump;
}

function incrementVersion(version, bump) {
  if (bump === "major") return { major: version.major + 1, minor: 0, patch: 0 };
  if (bump === "minor") return { major: version.major, minor: version.minor + 1, patch: 0 };
  return { major: version.major, minor: version.minor, patch: version.patch + 1 };
}

function parseRequestedMajor(value) {
  if (value === null || value === undefined || value === "") return null;
  const text = String(value).trim();
  if (!/^[1-9]\d*$/.test(text)) throw new Error(".github/release-major-version must contain a positive integer");
  return Number(text);
}

function buildReleasePlan({ tags, mergedTags, messages, channel, requestedMajor = null }) {
  if (channel !== "stable" && channel !== "rc") throw new Error(`unsupported release channel: ${channel}`);
  const latestStable = findLatestStableTag(tags);
  const stableVersion = latestStable ? latestStable.version : { major: 0, minor: 0, patch: 0 };
  const explicitMajor = parseRequestedMajor(requestedMajor);
  const requestedMajorVersion = explicitMajor && explicitMajor > stableVersion.major ? { major: explicitMajor, minor: 0, patch: 0 } : null;
  const conventionalBump = requiredBump(messages);
  const automaticBump = conventionalBump === "major" ? "minor" : conventionalBump;
  const automaticRelease = automaticBump !== "none";
  const nextStable = requestedMajorVersion || (automaticRelease ? incrementVersion(stableVersion, automaticBump) : stableVersion);
  const nextStableText = formatVersion(nextStable);
  const publish = requestedMajorVersion !== null || (channel === "stable" ? automaticRelease : automaticBump === "minor");

  if (channel === "stable") {
    return { channel, publish, latestStableTag: latestStable ? latestStable.tag : "", previousTag: latestStable ? latestStable.tag : "", stableVersion: nextStableText, newVersion: nextStableText, newTag: `v${nextStableText}`, rcNumber: "", releaseName: `Release v${nextStableText}` };
  }

  const activeCandidates = mergedTags.map(parseReleaseCandidate)
    .filter((candidate) => candidate && compareVersions(candidate.version, stableVersion) > 0)
    .sort((left, right) => right.number - left.number);
  const previousCandidate = activeCandidates[0] || null;
  const rcNumber = previousCandidate ? previousCandidate.number + 1 : 1;
  const newVersion = `${nextStableText}-rc.${rcNumber}`;
  return { channel, publish, latestStableTag: latestStable ? latestStable.tag : "", previousTag: previousCandidate ? previousCandidate.tag : latestStable ? latestStable.tag : "", stableVersion: nextStableText, newVersion, newTag: `v${newVersion}`, rcNumber: String(rcNumber), releaseName: `Release v${nextStableText} RC${rcNumber}` };
}

function git(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function main() {
  const channelArgument = process.argv.find((argument) => argument.startsWith("--channel="));
  const channel = channelArgument ? channelArgument.slice("--channel=".length) : "";
  const tags = git(["tag", "--list"]).split(/\r?\n/).filter(Boolean);
  const mergedTags = git(["tag", "--merged", "HEAD"]).split(/\r?\n/).filter(Boolean);
  const latestStable = findLatestStableTag(tags);
  const messages = git(["log", latestStable ? `${latestStable.tag}..HEAD` : "HEAD", "--format=%B%x00"])
    .split("\0").map((message) => message.trim()).filter(Boolean);
  const majorFile = ".github/release-major-version";
  const requestedMajor = existsSync(majorFile) ? readFileSync(majorFile, "utf8") : null;
  const plan = buildReleasePlan({ tags, mergedTags, messages, channel, requestedMajor });
  const outputNames = { publish: String(plan.publish), latest_stable_tag: plan.latestStableTag, previous_tag: plan.previousTag, stable_version: plan.stableVersion, new_version: plan.newVersion, new_tag: plan.newTag, rc_number: plan.rcNumber, release_name: plan.releaseName };
  if (process.env.GITHUB_OUTPUT) {
    for (const [name, value] of Object.entries(outputNames)) appendFileSync(process.env.GITHUB_OUTPUT, `${name}=${value}\n`);
  }
  process.stdout.write(`${JSON.stringify(plan)}\n`);
}

module.exports = { buildReleasePlan, findLatestStableTag, parseRequestedMajor, requiredBump };
if (require.main === module) main();
