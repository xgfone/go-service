package service

// FailHandler is used to calculate the index of the next endpoint to be used
// to retry the service. If returning -1, it will terminate to retry.
type FailHandler func(currentEndpointIndex, hasRetriedNum int) (nextEndpointIndex int)

// FailFast returns a fast fail handler, which returns the error instantly
// and no retry.
func FailFast() FailHandler { return func(index, retry int) int { return -1 } }

// FailTry returns a fail handler, which will retry the same endpoint
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry the same endpoint for the number
// of the endpoints.
func FailTry(maxnum int) FailHandler {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return func(index, retry int) int {
		if maxnum > 0 && retry > maxnum {
			return -1
		}
		return index
	}
}

// FailOver returns a fail handler, which will retry the other endpoints
// until the maximum retry number.
//
// If maxnum is equal to 0, it will retry until all endpoints are retryied.
func FailOver(maxnum int) FailHandler {
	if maxnum < 0 {
		panic("the retry maximum number must not be a negative integer")
	}

	return func(index, retry int) int {
		if maxnum > 0 && retry > maxnum {
			return -1
		}
		return index + 1
	}
}
