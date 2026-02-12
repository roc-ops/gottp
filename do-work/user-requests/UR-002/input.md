---
id: UR-002
title: Inner group TTP compatibility bug and existing test failures
created_at: 2026-02-12T17:32:54Z
requests: [REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
word_count: 350
---

# Inner group TTP compatibility bug and existing test failures

## Summary

User reports that gottp produces different results than Python TTP when a template uses an inner group inside a named group with `method="table"`. Removing the inner group makes gottp match Python output; with the inner group gottp does not match. User provided a full template demonstrating the issue. Additionally, user requests that the 4 currently failing tests from the test suite be captured as individual do-work requests.

## Extracted Requests

| ID | Title | Summary |
|----|-------|---------|
| REQ-002 | Fix inner group output mismatch with Python TTP | Diagnose and fix inner group handling difference, add test case |
| REQ-003 | Fix TestSet/set_field failure | set() group function doesn't produce expected new_field |
| REQ-004 | Fix TestSet/single_arg failure | set() with single arg should error but returns nil |
| REQ-005 | Fix TestResub/multiple_matches failure | resub() only replaces first match instead of all |
| REQ-006 | Fix TestStartEndIndicatorTogether failure | Combined start/end indicator produces empty result |

## Batch Constraints

- The inner group bug (REQ-002) is the primary request — user wants diagnosis, fix, and a test case
- The 4 test failure REQs (REQ-003 through REQ-006) were requested as tracking items for known failures from the test suite run
- All fixes should include appropriate test coverage

## Full Verbatim Input

when I run this template with Python TTP, I get different results than gottp, if I remove the inner group gottp matches python, with inner group but still does not match its output with both. <template>
<macro>
import ipaddress
from datetime import datetime

def to_string_ip(data):
    """Custom function to convert an IP address object to a string."""
    if isinstance(data, (ipaddress.IPv4Address, ipaddress.IPv6Address)):
        return str(data)
    return data

# Any custom defs go here
</macro>
<lookup name="ifTypes" load="json">
{

    "ifType_ethernet_csmacd": "ethernetCsmacd",
    "ifType_CMTSmac": "docsCableMaclayer",
    "ifType_CMTSDownStream": "docsCableDownstream",
    "ifType_CMTSUPStream_physical": "docsCableUpstream",
    "ifType_CMTSUPstream_logic": "docsCableUpstreamChannel",
    "ifType_l3ipvlan": "l3ipvlan",
    "ifType_softwareLoopback": "softwareLoopback",
    "IfType_CMTSVideoDownstream": "docsCableDownstream",
    "ifType_CMTSDOWNstream_RfPort": "cableDownstreamRfPort",
    "ifType_CMTSOfdmDownstream": "docsOfdmDownstream",
    "ifType_ipForward": "ipForward",
    "ifType_ofdma": "docsCableUpstreamChannel"

}
</lookup>
<group name="yang.if-mib:IF-MIB.ifTable.ifEntry*" method="table">
<group>
ifIndex: {{ ifIndex | DIGIT | to_int  | _start_}}
ifDescr: {{ ifDescr | ORPHRASE }}
ifType: {{ ifType | ORPHRASE | rlookup('ifTypes')}}
ifMtu: {{ ifMtu| DIGIT | to_int }}
ifSpeed: {{ ifSpeed | DIGIT | to_int }}
ifPhysAddress: {{ ifPhysAddress | ORPHRASE }}
ifAdminStatus: {{ ifAdminStatus | ORPHRASE | lower }}({{ignore}})
ifOperStatus: {{ ifOperStatus | ORPHRASE | lower }}({{ignore}})
ifLastChange: {{ ifLastChange | ORPHRASE }}
ifInOctets: {{ ifInOctets | DIGIT | to_int }}
ifHCInOctets: {{ ifHCInOctets | DIGIT | to_int }}
ifInUcastPkts: {{ ifInUcastPkts | DIGIT | to_int }}
ifHCInUcastPkts: {{ ifHCInUcastPkts | DIGIT | to_int }}
ifInDiscards: {{ ifInDiscards | DIGIT | to_int }}
ifInErrors: {{ ifInErrors | DIGIT | to_int }}
ifInUnknownProtos: {{ ifInUnknownProtos | DIGIT | to_int }}
ifOutOctets: {{ ifOutOctets | DIGIT | to_int }}
ifHCOutOctets: {{ ifHCOutOctets | DIGIT | to_int }}
ifOutUcastPkts: {{ ifOutUcastPkts | DIGIT | to_int }}
ifHCOutUcastPkts: {{ ifHCOutUcastPkts | DIGIT | to_int }}
ifOutDiscards: {{ ifOutDiscards | DIGIT | to_int }}
ifOutErrors: {{ ifOutErrors | DIGIT | to_int }}
ifName: {{ ifName | ORPHRASE }}
ifInMulticastPkts: {{ ifInMulticastPkts | DIGIT | to_int }}
ifHCInMulticastPkts: {{ ifHCInMulticastPkts | DIGIT | to_int }}
ifInBroadcastPkts: {{ ifInBroadcastPkts | DIGIT | to_int }}
ifHCInBroadcastPkts: {{ ifHCInBroadcastPkts | DIGIT | to_int }}
ifOutMulticastPkts: {{ ifOutMulticastPkts | DIGIT | to_int }}
ifHCOutMulticastPkts: {{ ifHCOutMulticastPkts | DIGIT | to_int }}
ifOutBroadcastPkts: {{ ifOutBroadcastPkts | DIGIT | to_int }}
ifHCOutBroadcastPkts: {{ ifHCOutBroadcastPkts | DIGIT | to_int }}
ifLinkUpDownTrapEnable: {{ ifLinkUpDownTrapEnable | ORPHRASE | lower }}
ifHighSpeed: {{ ifHighSpeed | DIGIT | to_int }}
ifPromiscuousMode: {{ ifPromiscuousMode | DIGIT | to_int }}
ifConnectorPresent: {{ ifConnectorPresent | DIGIT | to_int }}
ifAlias: {{ ifAlias | ORPHRASE }}
ifCounterDiscontinuityTime: {{ ifCounterDiscontinuityTime | ORPHRASE }}
 can we determine what the issue is, fix it and add a test case to confirm going forward, also any tests that currently fail can we setup /do-work requests for them also

## Addendum (2026-02-12T17:33Z) — Sample Input Data

User provided sample input data for the test:

```
r3r2-c100g>show iftable detail
total interface number is 894

------------------------------------------------
ifIndex: 1
ifDescr: eth 6/0
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 100000000
ifPhysAddress: 00:17:10:2b:30:82
ifAdminStatus: Up(1)
ifOperStatus: Up(1)
ifLastChange: 0 day 00h:01m:30s.60th
ifInOctets: 228941791
ifHCInOctets: 228941791
ifInUcastPkts: 3454207
ifHCInUcastPkts: 3454207
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 1266
ifHCOutOctets: 1266
ifOutUcastPkts: 11
ifHCOutUcastPkts: 11
ifOutDiscards: 0
ifOutErrors: 0
ifName: eth 6/0
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 100
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000072
ifDescr: XGige 6/0
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:81
ifAdminStatus: Up(1)
ifOperStatus: Up(1)
ifLastChange: 23 day 21h:15m:58s.62th
ifInOctets: 945984277
ifHCInOctets: 945984277
ifInUcastPkts: 1099491
ifHCInUcastPkts: 1099491
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 296867
ifOutOctets: 137590080
ifHCOutOctets: 137590080
ifOutUcastPkts: 649688
ifHCOutUcastPkts: 649688
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/0
ifInMulticastPkts: 1726049
ifHCInMulticastPkts: 1726049
ifInBroadcastPkts: 48288
ifHCInBroadcastPkts: 48288
ifOutMulticastPkts: 97545
ifHCOutMulticastPkts: 97545
ifOutBroadcastPkts: 65446
ifHCOutBroadcastPkts: 65446
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000073
ifDescr: XGige 6/1
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:82
ifAdminStatus: Down(2)
ifOperStatus: Down(2)
ifLastChange: 0 day 00h:00m:00s.00th
ifInOctets: 0
ifHCInOctets: 0
ifInUcastPkts: 0
ifHCInUcastPkts: 0
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 0
ifHCOutOctets: 0
ifOutUcastPkts: 0
ifHCOutUcastPkts: 0
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/1
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000074
ifDescr: XGige 6/2
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:83
ifAdminStatus: Down(2)
ifOperStatus: Down(2)
ifLastChange: 0 day 00h:00m:00s.00th
ifInOctets: 0
ifHCInOctets: 0
ifInUcastPkts: 0
ifHCInUcastPkts: 0
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 0
ifHCOutOctets: 0
ifOutUcastPkts: 0
ifHCOutUcastPkts: 0
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/2
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
```

---
*Captured: 2026-02-12T17:32:54Z*
