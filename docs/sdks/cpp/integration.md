# C++ SDK Integration Guide

This document explains how to integrate the C++ SDK into an existing project.

## Windows Integration Tips

- Set include directories and library directories explicitly
- The runtime library setting must match the triplet
- Verify dependencies separately for Debug and Release builds

## Key Points

- The legacy gRPC local-listener model is no longer the default assumption
- Build the integration around the unified session model first
