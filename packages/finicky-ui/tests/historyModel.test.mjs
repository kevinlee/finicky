import assert from "node:assert/strict";
import test from "node:test";
import { historyOpenURL } from "../src/lib/historyModel.ts";

test("history prefers the cleaned web URL when it remains routable", () => {
  assert.equal(
    historyOpenURL({
      url: "https://example.com/clean",
      originalUrl: "https://example.com/clean?utm_source=email",
    }),
    "https://example.com/clean",
  );
  assert.equal(
    historyOpenURL({ url: "file:///tmp/document.html" }),
    "file:///tmp/document.html",
  );
});

test("native app history reopens through the original web link", () => {
  for (const url of [
    "spotify:track:123",
    "msteams://teams.microsoft.com/l/meetup-join/example",
    "linear://team/issue/ID-1",
  ]) {
    assert.equal(
      historyOpenURL({ url, originalUrl: "https://example.com/original" }),
      "https://example.com/original",
    );
  }
});

test("history does not reopen unsupported or malformed URLs without a valid source", () => {
  assert.equal(historyOpenURL({ url: "spotify:track:123" }), null);
  assert.equal(
    historyOpenURL({ url: "not a URL", originalUrl: "javascript:alert(1)" }),
    null,
  );
});
