package domain

import "time"

const (
	MicrosPerDollar = 1_000_000

	DefaultAPIVersion = "v1"
	DefaultHTTPAddr   = ":8080"
	DefaultCORSOrigin = "http://localhost:5173"

	EnvDatabaseURL   = "DATABASE_URL"
	EnvHTTPAddr      = "HTTP_ADDR"
	EnvAPIVersion    = "API_VERSION"
	EnvKeyPepper     = "KEY_PEPPER"
	EnvOpsToken      = "OPS_TOKEN"
	EnvWebhookSecret = "WEBHOOK_SECRET"
	EnvCORSOrigins   = "CORS_ORIGINS"

	ReadHeaderTimeout = 5 * time.Second
	ShutdownTimeout   = 5 * time.Second
	WorkerTick        = 5 * time.Second
	CORSMaxAge        = "600"

	MaxEventBatch   = 500
	MaxRequestIDLen = 128
	APIKeyPrefix    = "sk_live_"
	APIKeyPrefixLen = 20
	BearerPrefix    = "Bearer "
	JSONContentType = "application/json"

	DefaultPageSize     = 50
	MaxUsagePage        = 200
	MaxOpsPage          = 200
	DefaultInvoicePage  = 20
	MaxInvoicePage      = 100
	OpsInvoiceListLimit = 20
	DefaultHourJobLimit = 100

	StatusIssued = "issued"
	StatusPaid   = "paid"

	LineKindTier   = "tier"
	LineKindCredit = "credit"
	CreditLineDesc = "Account credit"

	ActorOps = "ops"

	AnomalyFactor = 10
	AvgWindowDays = 30

	StandardPlanID = "11111111-1111-1111-1111-111111111111"

	SeedNamePrefix     = "seed:%"
	SeedHourBatch      = 500
	SeedHourPasses     = 200
	SeedFlushChunk     = 200
	SeedJulyFloorUnits = 25000
	SeedSpikeUnits     = 8000

	TestKeyPepper   = "dev-key-pepper-change-me"
	TestDatabaseURL = "postgres://app:app@localhost:5432/billing?sslmode=disable"

	SeedHarborline = "seed:Harborline"
	SeedCinder     = "seed:Cinder"
	SeedQuill      = "seed:Quill"
	SeedKestrel    = "seed:Kestrel"

	MsgUnauthorized          = "unauthorized"
	MsgNotFound              = "not found"
	MsgInvalidJSON           = "invalid json"
	MsgInvalidBody           = "invalid body"
	MsgTimestampRFC3339      = "timestamp must be RFC3339"
	MsgFromToRFC3339         = "from and to must be RFC3339"
	MsgInvoicePaid           = "invoice is paid"
	MsgOriginNotAllowed      = "origin not allowed"
	MsgDBDown                = "db down"
	MsgEmptyBatch            = "empty batch"
	MsgBatchTooLarge         = "batch too large"
	MsgRequestIDRequired     = "request_id is required"
	MsgRequestIDTooLong      = "request_id too long"
	MsgEndpointRequired      = "endpoint is required"
	MsgUnitsMustBePositive   = "units must be > 0"
	MsgTimestampRequired     = "timestamp is required"
	MsgAmountMustBePositive  = "amount must be > 0"
	MsgReasonAndIdempotency  = "reason and Idempotency-Key are required"
	MsgReasonRequired        = "reason is required"
	MsgEventAndInvoiceID     = "event_id and invoice_id are required"
	MsgFromAndToRequired     = "from and to are required"
	MsgToAfterFrom           = "to must be after from"
	MsgBadCursor             = "bad cursor"
	MsgEventStoreMissing     = "event store is missing"
	MsgWindowStoreMissing    = "window store is missing"
	MsgInvoiceStoreMissing   = "invoice store is missing"
	MsgUsageStoreMissing     = "usage store is missing"
	MsgAuthNotConfigured     = "auth is not configured"
	MsgRequiredSuffix        = " is required"
	MsgDatabaseURLRequired   = EnvDatabaseURL + MsgRequiredSuffix
	MsgKeyPepperRequired     = EnvKeyPepper + MsgRequiredSuffix
	MsgOpsTokenRequired      = EnvOpsToken + MsgRequiredSuffix
	MsgWebhookSecretRequired = EnvWebhookSecret + MsgRequiredSuffix
	MsgDirtyHoursLeft        = "dirty hours still left after seed"
	MsgDBConnect             = "db connect"
	MsgDBPing                = "db ping"

	MsgSeedAlreadyRan = "seed already ran, skip"
	MsgSeedAPIKeys    = "API keys (plaintext once):"
	MsgSeedKeyLine    = "  %s key%d  %s\n"
	MsgSeedDone       = "seed done, invoices issued for previous month: %d"
	FmtSeedKeyName    = "key-%d"

	MsgHourJobErr       = "hour job: %v"
	MsgHourJobProcessed = "hour job processed %d dirty hours"
	MsgInvoiceJobErr    = "invoice job: %v"
	MsgInvoiceJobIssued = "invoice job issued %d"
	MsgWorkerUp         = "worker up"
	MsgWorkerStopping   = "worker stopping"
	MsgAPIListening     = "api listening on %s (%s)"

	FmtTierOpen   = "%d units over %d"
	FmtTierClosed = "%d units (%d-%d)"
)

type SeedFirm struct {
	Name  string
	Keys  int
	Busy  bool
	Spike bool
}

var SeedFirms = []SeedFirm{
	{Name: SeedHarborline, Keys: 2, Busy: true},
	{Name: SeedCinder, Keys: 1, Busy: true},
	{Name: SeedQuill, Keys: 1},
	{Name: SeedKestrel, Keys: 1, Busy: true, Spike: true},
}

var SeedEndpoints = []string{"/v1/translate", "/v1/search", "/v1/embed"}
