package audit

// AuditOption is a function that modifies an audit record
type AuditOption func(*AuditRecord)

// WithActor sets the actor information
func WithActor(actor ActorInfo) AuditOption {
	return func(r *AuditRecord) {
		r.Actor = actor
	}
}

// WithActorID sets the actor ID
func WithActorID(id, actorType, name string) AuditOption {
	return func(r *AuditRecord) {
		r.Actor = ActorInfo{
			ID:   id,
			Type: actorType,
			Name: name,
		}
	}
}

// WithResource sets the resource information
func WithResource(resource ResourceInfo) AuditOption {
	return func(r *AuditRecord) {
		r.Resource = resource
	}
}

// WithResourceID sets the resource ID
func WithResourceID(resourceType, id string) AuditOption {
	return func(r *AuditRecord) {
		r.Resource = ResourceInfo{
			Type: resourceType,
			ID:   id,
		}
	}
}

// WithDetails sets the details
func WithDetails(details map[string]interface{}) AuditOption {
	return func(r *AuditRecord) {
		r.Details = details
	}
}

// WithChanges sets the change information
func WithChanges(before, after map[string]interface{}) AuditOption {
	return func(r *AuditRecord) {
		r.Changes = &ChangeInfo{
			Before: before,
			After:  after,
		}
		// Calculate diff fields
		r.Changes.DiffFields = calculateDiffFields(before, after)
	}
}

// WithContext sets the audit context
func WithContext(ctx AuditContext) AuditOption {
	return func(r *AuditRecord) {
		r.Context = ctx
	}
}

// WithOutcome sets the outcome
func WithOutcome(outcome, errMsg string) AuditOption {
	return func(r *AuditRecord) {
		r.Outcome = outcome
		r.ErrorMessage = errMsg
	}
}

// WithSeverity sets the severity
func WithSeverity(severity AuditSeverity) AuditOption {
	return func(r *AuditRecord) {
		r.Severity = severity
	}
}

// WithCategory sets the category
func WithCategory(category AuditCategory) AuditOption {
	return func(r *AuditRecord) {
		r.Category = category
	}
}

// WithGameID sets the game ID
func WithGameID(gameID, env string) AuditOption {
	return func(r *AuditRecord) {
		r.Resource.GameID = gameID
		r.Resource.Environment = env
	}
}

// WithIPAddress sets the IP address
func WithIPAddress(ip, userAgent string) AuditOption {
	return func(r *AuditRecord) {
		r.Actor.IPAddress = ip
		r.Actor.UserAgent = userAgent
	}
}
