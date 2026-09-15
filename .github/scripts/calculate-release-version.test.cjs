const assert = require("node:assert/strict");
const { buildReleasePlan, findLatestStableTag, parseRequestedMajor, requiredBump } = require("./calculate-release-version.cjs");

assert.equal(findLatestStableTag(["v0.2.9", "v0.10.0", "v0.10.1-rc.1"]).tag, "v0.10.0");
assert.equal(requiredBump(["fix: correct behavior"]), "patch");
assert.equal(requiredBump(["fix: correct behavior", "feat(ui): add setup"]), "minor");
assert.equal(requiredBump(["feat(api)!: replace response"]), "major");
assert.equal(requiredBump(["feat(api): replace response\n\nBREAKING CHANGE: response fields changed"]), "major");
assert.equal(parseRequestedMajor("2\n"), 2);
assert.equal(parseRequestedMajor(""), null);
assert.throws(() => parseRequestedMajor("v2.0.0"), /positive integer/);

assert.equal(buildReleasePlan({ tags: ["v0.2.1"], mergedTags: ["v0.2.1"], messages: ["feat(api)!: replace response"], channel: "rc" }).newTag, "v0.3.0-rc.1");
assert.equal(buildReleasePlan({ tags: ["v0.2.1"], mergedTags: ["v0.2.1"], messages: ["fix: prepare milestone"], channel: "rc", requestedMajor: "1" }).newTag, "v1.0.0-rc.1");
assert.equal(buildReleasePlan({ tags: ["v1.0.0"], mergedTags: ["v1.0.0"], messages: ["fix: next cycle"], channel: "stable", requestedMajor: "1" }).newTag, "v1.0.1");
assert.equal(buildReleasePlan({ tags: ["v0.2.1"], mergedTags: ["v0.2.1"], messages: ["fix: behavior"], channel: "rc" }).newTag, "v0.2.2-rc.1");
assert.equal(buildReleasePlan({ tags: ["v0.2.1", "v0.2.2-rc.1"], mergedTags: ["v0.2.1", "v0.2.2-rc.1"], messages: ["feat: capability"], channel: "rc" }).newTag, "v0.3.0-rc.2");
assert.equal(buildReleasePlan({ tags: ["v0.2.2-rc.1", "v0.3.0-rc.2", "v0.3.0"], mergedTags: ["v0.2.2-rc.1", "v0.3.0-rc.2"], messages: ["fix: next cycle"], channel: "rc" }).newTag, "v0.3.1-rc.1");
assert.equal(buildReleasePlan({ tags: ["v0.2.1", "v0.3.0-rc.1"], mergedTags: ["v0.2.1", "v0.3.0-rc.1"], messages: ["feat: capability"], channel: "stable" }).newTag, "v0.3.0");

console.log("release version tests passed");
