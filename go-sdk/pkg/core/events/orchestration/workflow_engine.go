package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DAGNode represents a node in the directed acyclic graph
type DAGNode struct {
	ID           string
	Stage        *ValidationStage
	Dependencies []*DAGNode
	Dependents   []*DAGNode
	Status       NodeStatus
	StartTime    time.Time
	EndTime      time.Time
	Error        error
	mu           sync.RWMutex
}

// NodeStatus represents the execution status of a DAG node
type NodeStatus int

const (
	NodePending NodeStatus = iota
	NodeReady
	NodeRunning
	NodeCompleted
	NodeFailed
	NodeSkipped
	NodeBlocked
)

// ExecutionPlan represents a planned execution order
type ExecutionPlan struct {
	Levels    [][]*DAGNode
	Parallel  map[string][]*DAGNode
	Critical  []*DAGNode
	Optional  []*DAGNode
	Estimated time.Duration
}

// WorkflowEngine executes validation workflows using DAG-based execution
type WorkflowEngine struct {
	orchestrator *Orchestrator
	executorPool *ExecutorPool
	mu           sync.RWMutex
}

// ExecutorPool manages concurrent stage execution
type ExecutorPool struct {
	workers   chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	taskQueue chan *ExecutionTask
}

// ExecutionTask represents a stage execution task
type ExecutionTask struct {
	Node      *DAGNode
	Context   *ValidationContext
	Result    *ValidationResult
	Callback  func(*StageResult, error)
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(orchestrator *Orchestrator) *WorkflowEngine {
	engine := &WorkflowEngine{
		orchestrator: orchestrator,
	}

	engine.executorPool = NewExecutorPool(orchestrator.config.MaxConcurrentWorkflows)
	return engine
}

// Execute executes a validation workflow using DAG-based execution
func (we *WorkflowEngine) Execute(ctx context.Context, workflow *ValidationWorkflow, validationCtx *ValidationContext, result *ValidationResult) error {
	// Build DAG from workflow stages
	dag, err := we.buildDAG(workflow)
	if err != nil {
		return fmt.Errorf("failed to build DAG: %w", err)
	}

	// Create execution plan
	plan, err := we.createExecutionPlan(dag, validationCtx)
	if err != nil {
		return fmt.Errorf("failed to create execution plan: %w", err)
	}

	// Execute plan
	return we.executePlan(ctx, plan, validationCtx, result)
}

// buildDAG constructs a directed acyclic graph from workflow stages
func (we *WorkflowEngine) buildDAG(workflow *ValidationWorkflow) (map[string]*DAGNode, error) {
	nodes := make(map[string]*DAGNode)

	// Create nodes
	for _, stage := range workflow.Stages {
		node := &DAGNode{
			ID:           stage.ID,
			Stage:        stage,
			Dependencies: make([]*DAGNode, 0),
			Dependents:   make([]*DAGNode, 0),
			Status:       NodePending,
		}
		nodes[stage.ID] = node
	}

	// Build dependencies
	for _, stage := range workflow.Stages {
		node := nodes[stage.ID]
		for _, depID := range stage.Dependencies {
			depNode, exists := nodes[depID]
			if !exists {
				return nil, fmt.Errorf("dependency not found: %s for stage %s", depID, stage.ID)
			}

			node.Dependencies = append(node.Dependencies, depNode)
			depNode.Dependents = append(depNode.Dependents, node)
		}
	}

	return nodes, nil
}

// createExecutionPlan creates an optimized execution plan
func (we *WorkflowEngine) createExecutionPlan(dag map[string]*DAGNode, validationCtx *ValidationContext) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		Levels:   make([][]*DAGNode, 0),
		Parallel: make(map[string][]*DAGNode),
		Critical: make([]*DAGNode, 0),
		Optional: make([]*DAGNode, 0),
	}

	// Topological sort to determine execution levels
	levels, err := we.topologicalSort(dag)
	if err != nil {
		return nil, fmt.Errorf("failed to create topological sort: %w", err)
	}

	plan.Levels = levels

	// Identify parallel execution opportunities
	for levelIdx, level := range levels {
		parallelGroups := we.identifyParallelGroups(level, validationCtx)
		plan.Parallel[fmt.Sprintf("level_%d", levelIdx)] = parallelGroups
	}

	// Identify critical path
	plan.Critical = we.identifyCriticalPath(dag)

	// Classify optional stages
	for _, node := range dag {
		if node.Stage.Optional {
			plan.Optional = append(plan.Optional, node)
		}
	}

	// Estimate execution time
	plan.Estimated = we.estimateExecutionTime(plan)

	return plan, nil
}

// topologicalSort performs topological sorting of the DAG
func (we *WorkflowEngine) topologicalSort(dag map[string]*DAGNode) ([][]*DAGNode, error) {
	// Calculate in-degrees
	inDegree := make(map[string]int)
	for nodeID, node := range dag {
		inDegree[nodeID] = len(node.Dependencies)
	}

	levels := make([][]*DAGNode, 0)
	processed := make(map[string]bool)

	for len(processed) < len(dag) {
		currentLevel := make([]*DAGNode, 0)

		// Find nodes with in-degree 0
		for nodeID, node := range dag {
			if !processed[nodeID] && inDegree[nodeID] == 0 {
				currentLevel = append(currentLevel, node)
			}
		}

		if len(currentLevel) == 0 {
			return nil, fmt.Errorf("circular dependency detected in DAG")
		}

		levels = append(levels, currentLevel)

		// Update in-degrees and mark as processed
		for _, node := range currentLevel {
			processed[node.ID] = true
			for _, dependent := range node.Dependents {
				inDegree[dependent.ID]--
			}
		}
	}

	return levels, nil
}

// identifyParallelGroups identifies stages that can run in parallel
func (we *WorkflowEngine) identifyParallelGroups(level []*DAGNode, validationCtx *ValidationContext) []*DAGNode {
	parallel := make([]*DAGNode, 0)

	for _, node := range level {
		// Check if stage allows parallel execution
		if node.Stage.Parallel {
			// Check if conditions are met for parallel execution
			if we.evaluateStageConditions(node.Stage, validationCtx) {
				parallel = append(parallel, node)
			}
		}
	}

	return parallel
}

// identifyCriticalPath identifies the critical path in the DAG
func (we *WorkflowEngine) identifyCriticalPath(dag map[string]*DAGNode) []*DAGNode {
	// Find longest path (critical path) using dynamic programming
	memo := make(map[string]time.Duration)
	path := make(map[string]*DAGNode)

	var findLongestPath func(*DAGNode) time.Duration
	findLongestPath = func(node *DAGNode) time.Duration {
		if duration, exists := memo[node.ID]; exists {
			return duration
		}

		maxDuration := time.Duration(0)
		var nextNode *DAGNode

		for _, dependent := range node.Dependents {
			duration := findLongestPath(dependent)
			if duration > maxDuration {
				maxDuration = duration
				nextNode = dependent
			}
		}

		nodeTime := node.Stage.Timeout
		if nodeTime == 0 {
			nodeTime = 30 * time.Second // Default estimate
		}

		totalDuration := nodeTime + maxDuration
		memo[node.ID] = totalDuration
		path[node.ID] = nextNode

		return totalDuration
	}

	// Find the starting nodes (nodes with no dependencies)
	startNodes := make([]*DAGNode, 0)
	for _, node := range dag {
		if len(node.Dependencies) == 0 {
			startNodes = append(startNodes, node)
		}
	}

	// Find the critical path
	var criticalStart *DAGNode
	maxCriticalTime := time.Duration(0)

	for _, node := range startNodes {
		duration := findLongestPath(node)
		if duration > maxCriticalTime {
			maxCriticalTime = duration
			criticalStart = node
		}
	}

	// Build critical path
	critical := make([]*DAGNode, 0)
	current := criticalStart
	for current != nil {
		critical = append(critical, current)
		current = path[current.ID]
	}

	return critical
}

// estimateExecutionTime estimates total execution time
func (we *WorkflowEngine) estimateExecutionTime(plan *ExecutionPlan) time.Duration {
	totalTime := time.Duration(0)

	for _, level := range plan.Levels {
		levelTime := time.Duration(0)
		for _, node := range level {
			stageTime := node.Stage.Timeout
			if stageTime == 0 {
				stageTime = 30 * time.Second
			}

			if node.Stage.Parallel && len(level) > 1 {
				// Parallel execution - take maximum time in level
				if stageTime > levelTime {
					levelTime = stageTime
				}
			} else {
				// Sequential execution - add times
				levelTime += stageTime
			}
		}
		totalTime += levelTime
	}

	return totalTime
}

// executePlan executes the planned workflow
func (we *WorkflowEngine) executePlan(ctx context.Context, plan *ExecutionPlan, validationCtx *ValidationContext, result *ValidationResult) error {
	nodeResults := make(map[string]*StageResult)
	var planMu sync.RWMutex

	// Execute levels sequentially
	for levelIdx, level := range plan.Levels {
		levelKey := fmt.Sprintf("level_%d", levelIdx)
		parallelNodes := plan.Parallel[levelKey]

		// Execute parallel nodes concurrently
		if len(parallelNodes) > 0 {
			err := we.executeParallelNodes(ctx, parallelNodes, validationCtx, result, nodeResults, &planMu)
			if err != nil {
				return err
			}
		}

		// Execute remaining nodes in level sequentially
		sequentialNodes := we.getSequentialNodes(level, parallelNodes)
		for _, node := range sequentialNodes {
			err := we.executeNode(ctx, node, validationCtx, result, nodeResults, &planMu)
			if err != nil && !node.Stage.Optional {
				return err
			}
		}

		// Check if we should continue based on level results
		if we.shouldStopExecution(level, nodeResults) {
			break
		}
	}

	return nil
}

// executeParallelNodes executes nodes in parallel
func (we *WorkflowEngine) executeParallelNodes(ctx context.Context, nodes []*DAGNode, validationCtx *ValidationContext, result *ValidationResult, nodeResults map[string]*StageResult, mu *sync.RWMutex) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(nodes))

	for _, node := range nodes {
		wg.Add(1)
		go func(n *DAGNode) {
			defer wg.Done()
			if err := we.executeNode(ctx, n, validationCtx, result, nodeResults, mu); err != nil && !n.Stage.Optional {
				errChan <- err
			}
		}(node)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// executeNode executes a single DAG node
func (we *WorkflowEngine) executeNode(ctx context.Context, node *DAGNode, validationCtx *ValidationContext, result *ValidationResult, nodeResults map[string]*StageResult, mu *sync.RWMutex) error {
	node.mu.Lock()
	node.Status = NodeRunning
	node.StartTime = time.Now()
	node.mu.Unlock()

	// Check dependencies
	if !we.dependenciesSatisfied(node, nodeResults, mu) {
		node.mu.Lock()
		node.Status = NodeBlocked
		node.mu.Unlock()
		return fmt.Errorf("dependencies not satisfied for stage: %s", node.ID)
	}

	// Check stage conditions
	if !we.evaluateStageConditions(node.Stage, validationCtx) {
		stageResult := &StageResult{
			StageID:    node.ID,
			Status:     Skipped,
			StartTime:  node.StartTime,
			EndTime:    time.Now(),
			Skipped:    true,
			SkipReason: "Stage conditions not met",
		}

		mu.Lock()
		nodeResults[node.ID] = stageResult
		result.StageResults[node.ID] = stageResult
		mu.Unlock()

		node.mu.Lock()
		node.Status = NodeSkipped
		node.EndTime = time.Now()
		node.mu.Unlock()

		return nil
	}

	// Execute stage using pipeline executor
	stageResult, err := we.orchestrator.pipelineExecutor.ExecuteStage(ctx, node.Stage, validationCtx)

	node.mu.Lock()
	node.EndTime = time.Now()
	if err != nil {
		node.Status = NodeFailed
		node.Error = err
	} else {
		node.Status = NodeCompleted
	}
	node.mu.Unlock()

	stageResult.Duration = node.EndTime.Sub(node.StartTime)

	mu.Lock()
	nodeResults[node.ID] = stageResult
	result.StageResults[node.ID] = stageResult
	mu.Unlock()

	return err
}

// dependenciesSatisfied checks if all dependencies are satisfied
func (we *WorkflowEngine) dependenciesSatisfied(node *DAGNode, nodeResults map[string]*StageResult, mu *sync.RWMutex) bool {
	mu.RLock()
	defer mu.RUnlock()

	for _, dep := range node.Dependencies {
		result, exists := nodeResults[dep.ID]
		if !exists || (result.Status != Completed && result.Status != Skipped) {
			return false
		}

		// If dependency failed and it's not optional, block execution
		if result.Status == Failed && !dep.Stage.Optional {
			return false
		}
	}

	return true
}

// evaluateStageConditions evaluates stage conditions
func (we *WorkflowEngine) evaluateStageConditions(stage *ValidationStage, validationCtx *ValidationContext) bool {
	for _, condition := range stage.Conditions {
		if !we.evaluateCondition(condition, validationCtx, nil) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (we *WorkflowEngine) evaluateCondition(condition StageCondition, validationCtx *ValidationContext, stageResults map[string]*StageResult) bool {
	var value interface{}

	switch condition.Type {
	case PropertyCondition:
		if validationCtx.Properties != nil {
			value = validationCtx.Properties[condition.Field]
		}
	case TagCondition:
		if validationCtx.Tags != nil {
			value = validationCtx.Tags[condition.Field]
		}
	case MetadataCondition:
		if validationCtx.Metadata != nil {
			value = validationCtx.Metadata[condition.Field]
		}
	case ResultCondition:
		if stageResults != nil {
			if result, exists := stageResults[condition.Field]; exists {
				value = result.Status
			}
		}
	}

	result := we.compareValues(value, condition.Operator, condition.Value)
	if condition.Negated {
		result = !result
	}

	return result
}

// compareValues compares two values using the specified operator
func (we *WorkflowEngine) compareValues(actual interface{}, operator ConditionOperator, expected interface{}) bool {
	if actual == nil && expected == nil {
		return operator == Equals
	}

	if actual == nil || expected == nil {
		return operator == NotEquals || operator == Exists
	}

	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	switch operator {
	case Equals:
		return actualStr == expectedStr
	case NotEquals:
		return actualStr != expectedStr
	case Contains:
		return fmt.Sprintf("%v", actual) != "" && 
			   fmt.Sprintf("%v", expected) != "" &&
			   len(actualStr) >= len(expectedStr) &&
			   actualStr != expectedStr
	case StartsWith:
		return len(actualStr) >= len(expectedStr) && actualStr[:len(expectedStr)] == expectedStr
	case EndsWith:
		return len(actualStr) >= len(expectedStr) && actualStr[len(actualStr)-len(expectedStr):] == expectedStr
	case Exists:
		return actual != nil
	default:
		return false
	}
}

// getSequentialNodes filters out parallel nodes to get sequential nodes
func (we *WorkflowEngine) getSequentialNodes(level []*DAGNode, parallelNodes []*DAGNode) []*DAGNode {
	parallelSet := make(map[string]bool)
	for _, node := range parallelNodes {
		parallelSet[node.ID] = true
	}

	sequential := make([]*DAGNode, 0)
	for _, node := range level {
		if !parallelSet[node.ID] {
			sequential = append(sequential, node)
		}
	}

	return sequential
}

// shouldStopExecution determines if execution should stop based on level results
func (we *WorkflowEngine) shouldStopExecution(level []*DAGNode, nodeResults map[string]*StageResult) bool {
	for _, node := range level {
		if result, exists := nodeResults[node.ID]; exists {
			if result.Status == Failed && node.Stage.OnFailure == StopPipeline {
				return true
			}
		}
	}
	return false
}

// NewExecutorPool creates a new executor pool
func NewExecutorPool(maxWorkers int) *ExecutorPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ExecutorPool{
		workers:   make(chan struct{}, maxWorkers),
		ctx:       ctx,
		cancel:    cancel,
		taskQueue: make(chan *ExecutionTask, maxWorkers*2),
	}

	// Initialize worker slots
	for i := 0; i < maxWorkers; i++ {
		pool.workers <- struct{}{}
	}

	// Start task processor
	go pool.processTasks()

	return pool
}

// processTasks processes execution tasks
func (ep *ExecutorPool) processTasks() {
	for {
		select {
		case task := <-ep.taskQueue:
			ep.wg.Add(1)
			go ep.executeTask(task)
		case <-ep.ctx.Done():
			return
		}
	}
}

// executeTask executes a single task
func (ep *ExecutorPool) executeTask(task *ExecutionTask) {
	defer ep.wg.Done()

	// Acquire worker
	<-ep.workers

	// Execute task
	// This would be implemented based on the specific stage execution logic
	// For now, we'll call the callback with a placeholder result

	defer func() {
		// Release worker
		ep.workers <- struct{}{}
	}()

	// TODO: Implement actual stage execution
	// This is a placeholder that would be replaced with actual execution logic
	result := &StageResult{
		StageID:   task.Node.ID,
		Status:    Completed,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	task.Callback(result, nil)
}

// Close shuts down the executor pool
func (ep *ExecutorPool) Close() {
	ep.cancel()
	ep.wg.Wait()
	close(ep.taskQueue)
}