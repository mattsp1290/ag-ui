# Dependency Analysis Report

```
Package Dependency Analysis
===========================

Core Package Dependencies:
--------------------------

github.com/ag-ui/go-sdk/pkg/core/events imports:
  - github.com/ag-ui/go-sdk/pkg/proto/generated
  - github.com/ag-ui/go-sdk/pkg/core

github.com/ag-ui/go-sdk/pkg/transport imports:
  - github.com/ag-ui/go-sdk/pkg/core/events
  - github.com/ag-ui/go-sdk/pkg/common

github.com/ag-ui/go-sdk/pkg/state imports:
  - github.com/ag-ui/go-sdk/pkg/common
  - github.com/ag-ui/go-sdk/pkg/core/events
  - github.com/ag-ui/go-sdk/pkg/testing

github.com/ag-ui/go-sdk/pkg/messages imports:
  - github.com/ag-ui/go-sdk/pkg/core/events

github.com/ag-ui/go-sdk/pkg/tools imports:
  - github.com/ag-ui/go-sdk/pkg/common


Circular Dependencies:
----------------------
No circular dependencies found!


Cross-Package Dependencies:
---------------------------

Transport -> Events:
  - github.com/ag-ui/go-sdk/pkg/core/events

State -> Events:
  - github.com/ag-ui/go-sdk/pkg/core/events

```
