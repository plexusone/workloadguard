# WorkloadGuard UI - Product Requirements Document

## Overview

WorkloadGuard is a macOS system health daemon that detects and mitigates runaway processes. This PRD defines the user interface requirements for making WorkloadGuard accessible to different user personas through multiple interface approaches.

## Problem Statement

The current WorkloadGuard implementation is CLI-only, requiring users to:
1. Know to run `workloadguard check` to see system state
2. Manually monitor logs or Prometheus metrics
3. Have technical knowledge to interpret output

Users need proactive, visual interfaces that alert them to system health issues and allow remediation without deep CLI knowledge.

## User Personas

### 1. Developer (Primary)
- Uses Claude Code, Cursor, or similar AI coding assistants
- Comfortable with terminal and CLI tools
- Wants minimal interruption, maximum signal
- Needs: Claude Code integration, TUI

### 2. Power User (Secondary)
- Uses PlexusOne Desktop for terminal management
- Wants integrated system monitoring
- Prefers native macOS experience
- Needs: PlexusOne Desktop integration

### 3. General User (Tertiary)
- Less technical, may not use terminal regularly
- Wants simple, visual system health monitoring
- Needs: Web dashboard, potential future Electron app

## User Stories

### Epic 1: Claude Code Integration

| ID | Story | Priority |
|----|-------|----------|
| CC-1 | As a developer, I want to see a warning in Claude Code when system load exceeds threshold | P0 |
| CC-2 | As a developer, I want to see which processes are candidates for termination | P0 |
| CC-3 | As a developer, I want to approve or skip termination from within Claude Code | P1 |
| CC-4 | As a developer, I want to configure thresholds via Claude Code settings | P2 |

### Epic 2: Terminal UI (TUI)

| ID | Story | Priority |
|----|-------|----------|
| TUI-1 | As a developer, I want to run `workloadguard top` to see real-time system state | P0 |
| TUI-2 | As a developer, I want to see process counts by policy with color-coded status | P0 |
| TUI-3 | As a developer, I want to terminate processes interactively from the TUI | P1 |
| TUI-4 | As a developer, I want to see historical trends (last 5 minutes) | P2 |

### Epic 3: PlexusOne Desktop Integration

| ID | Story | Priority |
|----|-------|----------|
| PD-1 | As a power user, I want to see system health in the PlexusOne status bar | P0 |
| PD-2 | As a power user, I want to see alerts when thresholds are exceeded | P0 |
| PD-3 | As a power user, I want a panel showing process details and policy status | P1 |
| PD-4 | As a power user, I want to terminate processes from the UI | P1 |
| PD-5 | As a power user, I want to configure WorkloadGuard from PlexusOne settings | P2 |

### Epic 4: Web Dashboard

| ID | Story | Priority |
|----|-------|----------|
| WD-1 | As a user, I want to access a dashboard at localhost:9090/dashboard | P1 |
| WD-2 | As a user, I want to see current load and process counts | P1 |
| WD-3 | As a user, I want to see policy status with visual indicators | P1 |
| WD-4 | As a user, I want the dashboard to auto-refresh | P2 |

## Functional Requirements

### FR-1: JSON API
The daemon must expose a JSON API for all UI clients:
- `GET /api/status` - Current system state
- `GET /api/policies` - Policy definitions and current evaluation
- `GET /api/processes/{name}` - Processes by name with parent info
- `POST /api/terminate` - Terminate specific PIDs
- `WebSocket /api/ws` - Real-time updates

### FR-2: Alert System
- Alerts must be triggered when any policy threshold is exceeded
- Alerts must include: policy name, process name, current count, threshold
- Alerts must indicate action being taken (notify, terminate, etc.)

### FR-3: Termination Confirmation
- Interactive UIs (TUI, PlexusOne, Web) should support manual termination
- Dry-run mode must be clearly indicated in all UIs

### FR-4: Configuration
- UIs should display current configuration
- Advanced UIs (PlexusOne, Web) may support configuration editing

## Non-Functional Requirements

### NFR-1: Performance
- API responses must complete in < 100ms
- WebSocket updates must be delivered within 1 second of state change
- TUI must not consume > 1% CPU when idle

### NFR-2: Reliability
- UI failures must not affect daemon operation
- Daemon must continue operating if no UI clients connected

### NFR-3: Security
- API must only listen on localhost by default
- No authentication required for localhost access
- Remote access (if enabled) must require authentication

## Success Metrics

| Metric | Target |
|--------|--------|
| Time to awareness (load spike to user notification) | < 5 seconds |
| User action time (notification to termination) | < 3 clicks/keypresses |
| API latency p99 | < 100ms |
| TUI memory usage | < 20MB |

## Out of Scope

- Mobile apps
- Cloud-hosted dashboard
- Multi-machine monitoring
- Windows/Linux support (macOS only)

## Dependencies

- WorkloadGuard daemon must be running
- For PlexusOne: PlexusOne Desktop app
- For Claude Code: Claude Code CLI with hooks support
