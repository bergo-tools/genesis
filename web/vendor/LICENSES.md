# Vendored frontend dependencies

These files are checked in so Genesis stays a zero-build, single-binary app
(no npm install, no bundler). They are served/embedded verbatim.

| File | Package | Version | License |
| --- | --- | --- | --- |
| vendor/preact.module.js | preact | 10.29.8 | MIT |
| vendor/hooks.module.js | preact/hooks | 10.29.8 | MIT |
| vendor/htm.module.js | htm | 3.1.1 | Apache-2.0 |
| ../test/render-to-string.module.js | preact-render-to-string | 6.7.0 | MIT |

Only change made to the upstream ESM builds: the bare `"preact"` specifier is
rewritten to a relative path, because browsers/Node cannot resolve bare
specifiers without an import map or bundler.

Upstream: https://github.com/preactjs/preact · https://github.com/developit/htm
