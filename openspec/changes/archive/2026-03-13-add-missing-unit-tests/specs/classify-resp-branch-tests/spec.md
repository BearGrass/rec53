## ADDED Requirements

### Requirement: classifyRespState handles NXDOMAIN negative response

#### Scenario: empty answer with SOA and NXDOMAIN rcode
- **WHEN** response has no Answer, SOA in Authority, and Rcode=NXDOMAIN
- **THEN** returns `CLASSIFY_RESP_GET_NEGATIVE` and caches the negative response

---

### Requirement: classifyRespState handles NODATA negative response

#### Scenario: empty answer with SOA and success rcode
- **WHEN** response has no Answer, SOA in Authority, and Rcode=NODATA (RcodeSuccess)
- **THEN** returns `CLASSIFY_RESP_GET_NEGATIVE` and caches the negative response

---

### Requirement: classifyRespState returns GET_NS when no answers and no SOA

#### Scenario: empty answer without SOA
- **WHEN** response has no Answer and no SOA in Authority
- **THEN** returns `CLASSIFY_RESP_GET_NS`

---

### Requirement: classifyRespState follows CNAME when qtype is not CNAME

#### Scenario: answer contains CNAME and qtype is A
- **WHEN** response Answer contains a CNAME and qtype != dns.TypeCNAME
- **THEN** returns `CLASSIFY_RESP_GET_CNAME`

#### Scenario: answer contains CNAME and qtype is CNAME
- **WHEN** qtype == dns.TypeCNAME and answer contains a CNAME record
- **THEN** does NOT return CLASSIFY_RESP_GET_CNAME (CNAME is the answer)

---

### Requirement: classifyRespState falls back to GET_NS when answers don't match qtype

#### Scenario: answers present but wrong type, no CNAME
- **WHEN** response has Answer records but none match qtype and no CNAME is present
- **THEN** returns `CLASSIFY_RESP_GET_NS`

---

### Requirement: classifyRespState returns error on nil input

#### Scenario: nil request or response
- **WHEN** request or response is nil
- **THEN** returns `CLASSIFY_RESP_COMMON_ERROR` with non-nil error
