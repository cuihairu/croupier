# TODO

## Service Model Coverage

The following API modules expose entities without corresponding GORM structs + DAO helpers under `services/server/internal/model`. For each item, add `<entity>.go` + `<entity>_model.go` (or equivalent) and register it in `model.AutoMigrate`.

- [x] `alert.api`: implement Alert & Silence models for alert management/muting resources.
- [x] `analytics_behavior.api`: add BehaviorEvent + FeatureAdoption structures to persist behavior analytics data.
- [x] `analytics_payments.api`: add ProductTrend + PaymentTransaction models for payment analytics.
- [x] `analytics_retention.api`: add RetentionCohort model for cohort retention tracking.
- [x] `backup.api`: add Backup model/DAO for snapshot records.
- [x] `faq.api`: add FAQ + FAQCategory models supporting knowledge base CRUD.
- [x] `feedback.api`: add Feedback model for user feedback flows.
- [x] `function.api`: add Function, FunctionDescriptor, Descriptor, FunctionInstance, FunctionPermission, PendingFunction models.
- [x] `node.api`: add Node + NodeCommand models for node inventory/execution history.
- [x] `profile.api`: add ProfilePermission + ProfileGame models (view tables for admin profile data).
- [x] `rate_limit.api`: add RateLimit model for throttling configurations.
- [x] `support.api`: add SupportTicket, SupportFAQ, SupportFeedback models.
- [x] `ticket.api`: add Ticket + Comment models for ticketing workflows.

Keep model/DAO files in `services/server/internal/model`, mirroring existing `admin.go` + `admin_model.go` pattern.
