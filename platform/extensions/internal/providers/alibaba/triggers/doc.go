// Package triggers documents Alibaba FC trigger types per the Trigger Capability Matrix.
//
// Implementations (EnsureHTTP, EnsureCron, EnsureQueue, EnsureStorage) live in the parent
// package alibaba/triggers.go because they use fcClient. This folder provides the trigger
// layout per matrix: http, cron, queue, storage.
package triggers
