package domain

// ProductActiveUserCount is a privacy-safe aggregate: Count is distinct users
// with at least one job in a window, without exposing the users themselves.
type ProductActiveUserCount struct {
	Surface   string
	Operation OperationType
	Modality  Modality
	Count     int64
}
