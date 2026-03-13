## Why

When users start rec53 with `./rec53 --config ./config.yaml`, the application crashes during startup. This is a critical issue that prevents users from running the DNS resolver with a configuration file, which is the primary recommended method of operation. Users should be able to reliably start the application with a valid config file without encountering runtime panics or fatal errors.

## What Changes

- Investigate and identify the root cause of the startup crash when using a config file
- Implement defensive error handling and validation in the startup sequence
- Add proper null checks and initialization guards for critical components
- Ensure graceful error reporting with actionable messages instead of crashes
- Verify that the application can safely initialize with various valid config file formats

## Capabilities

### New Capabilities
- `config-validation`: Validate config file format and required fields before initialization
- `startup-robustness`: Implement crash-resistant startup sequence with proper error handling

### Modified Capabilities
- `dns-server-startup`: Improve error handling and initialization safety to prevent crashes during server startup

## Impact

- Affected code: `cmd/rec53.go` (config loading and server initialization), `server/server.go` (server startup)
- Affected components: DNS server initialization, config parsing, monitor/logger setup
- User-facing impact: Users will be able to reliably start rec53 with config files
- No API changes or breaking changes
