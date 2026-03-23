package monitor

const (
	StateMachineStateInit     = "state_init"
	StateMachineHostsLookup   = "hosts_lookup"
	StateMachineForwardLookup = "forward_lookup"
	StateMachineCacheLookup   = "cache_lookup"
	StateMachineClassifyResp  = "classify_resp"
	StateMachineExtractGlue   = "extract_glue"
	StateMachineLookupNSCache = "lookup_ns_cache"
	StateMachineQueryUpstream = "query_upstream"
	StateMachineReturnResp    = "return_resp"
	StateMachineUnknownState  = "unknown"
	StateMachineSuccessExit   = "success_exit"
	StateMachineFormerrExit   = "formerr_exit"
	StateMachineServfailExit  = "servfail_exit"
	StateMachineErrorExit     = "error_exit"
	StateMachineMaxItersExit  = "max_iterations_exit"
)

var StateMachineCanonicalStates = []string{
	StateMachineStateInit,
	StateMachineHostsLookup,
	StateMachineForwardLookup,
	StateMachineCacheLookup,
	StateMachineClassifyResp,
	StateMachineExtractGlue,
	StateMachineLookupNSCache,
	StateMachineQueryUpstream,
	StateMachineReturnResp,
	StateMachineUnknownState,
}

var StateMachineCanonicalTerminals = []string{
	StateMachineSuccessExit,
	StateMachineFormerrExit,
	StateMachineServfailExit,
	StateMachineErrorExit,
	StateMachineMaxItersExit,
}
