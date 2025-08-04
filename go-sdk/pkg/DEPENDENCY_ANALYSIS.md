# Dependency Analysis Report

```
Package Dependency Analysis
===========================

Core Package Dependencies:
--------------------------

github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events imports:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core

github.com/mattsp1290/ag-ui/go-sdk/pkg/transport imports:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/common

github.com/mattsp1290/ag-ui/go-sdk/pkg/state imports:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/common
  - github.com/mattsp1290/ag-ui/go-sdk/internal/timeconfig
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/testing

github.com/mattsp1290/ag-ui/go-sdk/pkg/messages imports:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events

github.com/mattsp1290/ag-ui/go-sdk/pkg/tools imports:
  - github.com/mattsp1290/ag-ui/go-sdk/internal/timeconfig
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/common


Circular Dependencies:
----------------------
No circular dependencies found!


Cross-Package Dependencies:
---------------------------

Transport -> Events:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events

State -> Events:
  - github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events

```
