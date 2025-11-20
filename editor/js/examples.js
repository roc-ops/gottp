/**
 * Built-in TTP Template Examples
 */

const TTP_EXAMPLES = {
    cisco_interfaces: {
        name: 'Cisco Interface Configuration',
        description: 'Parse Cisco interface configurations with IP addresses and descriptions',
        data: `interface Loopback0
 ip address 192.168.0.113/24
 description Router-id-loopback
!
interface Vlan778
 ip address 2002::fd37/124
 description CPE_Acces_Vlan
!
interface GigabitEthernet0/0/0
 ip address 10.0.0.1/24
 description Uplink to Core
!
interface GigabitEthernet0/0/1
 ip address 172.16.0.1/24
 description Access Port
!`,
        template: `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>`
    },
    
    routing_table: {
        name: 'Routing Table',
        description: 'Parse routing table entries',
        data: `Codes: L - local, C - connected, S - static, R - RIP, M - mobile, B - BGP
       D - EIGRP, EX - EIGRP external, O - OSPF, IA - OSPF inter area
       N1 - OSPF NSSA external type 1, N2 - OSPF NSSA external type 2
       E1 - OSPF external type 1, E2 - OSPF external type 2
       i - IS-IS, su - IS-IS summary, L1 - IS-IS level-1, L2 - IS-IS level-2
       ia - IS-IS inter area, * - candidate default, U - per-user static route
       o - ODR, P - periodic downloaded static route, H - NHRP, l - LISP
       a - application route
       + - replicated route, % - next hop override

Gateway of last resort is 192.168.1.1 to network 0.0.0.0

S*    0.0.0.0/0 [1/0] via 192.168.1.1
      10.0.0.0/8 is variably subnetted, 2 subnets, 2 masks
C        10.0.0.0/24 is directly connected, GigabitEthernet0/0
L        10.0.0.1/32 is directly connected, GigabitEthernet0/0
      172.16.0.0/16 is variably subnetted, 2 subnets, 2 masks
C        172.16.0.0/24 is directly connected, Vlan100
L        172.16.0.1/32 is directly connected, Vlan100
      192.168.1.0/24 is variably subnetted, 2 subnets, 2 masks
C        192.168.1.0/24 is directly connected, GigabitEthernet0/1
L        192.168.1.1/32 is directly connected, GigabitEthernet0/1`,
        template: `<group name="routes">
{{ protocol }}\s+{{ network }}\s+\[{{ admin_distance }}/{{ metric }}\]\s+via\s+{{ next_hop }}
</group>`
    },
    
    system_logs: {
        name: 'System Log Parsing',
        description: 'Parse system log entries with timestamps and severity levels',
        data: `2024-01-15 10:23:45 INFO System startup completed
2024-01-15 10:24:12 WARNING High CPU usage detected: 85%
2024-01-15 10:25:30 ERROR Database connection failed
2024-01-15 10:26:45 INFO Backup completed successfully
2024-01-15 10:27:20 ERROR Disk space low: 5% remaining
2024-01-15 10:28:10 INFO User login: admin@example.com`,
        template: `<group name="log_entries">
{{ timestamp }}\s+{{ level }}\s+{{ message }}
</group>`
    },
    
    network_inventory: {
        name: 'Network Device Inventory',
        description: 'Extract device information from show commands',
        data: `Hostname: router-01.example.com
Model: Cisco IOS XE Software, Version 16.09.04
Serial Number: FTX12345678
Uptime: 45 days, 12 hours, 30 minutes
Memory: 2048 MB total, 512 MB used
CPU: 2 cores, 15% utilization
Interfaces: 24 GigabitEthernet, 4 TenGigabitEthernet`,
        template: `<group name="device_info">
Hostname:\s+{{ hostname }}
Model:\s+{{ model }}
Serial Number:\s+{{ serial_number }}
Uptime:\s+{{ uptime }}
Memory:\s+{{ memory_total }}\s+MB total,\s+{{ memory_used }}\s+MB used
CPU:\s+{{ cpu_cores }}\s+cores,\s+{{ cpu_utilization }}% utilization
Interfaces:\s+{{ interfaces }}
</group>`
    },
    
    bgp_neighbors: {
        name: 'BGP Neighbors',
        description: 'Parse BGP neighbor information',
        data: `BGP neighbor is 10.0.0.2, remote AS 65001, external link
  BGP version 4, remote router ID 10.0.0.2
  BGP state = Established, up for 2d 5h
  Last read 00:00:05, last write 00:00:03, hold time is 180, keepalive interval is 60 seconds
  Neighbor capabilities:
    Route refresh: advertised and received(new)
    Address family IPv4 Unicast: advertised and received
  Message statistics:
    InQ depth is 0
    OutQ depth is 0
                         Sent       Rcvd
    Opens:                  1          1
    Notifications:          0          0
    Updates:               15         12
    Keepalives:          3456       3458
    Route Refresh:          0          0
    Total:               3472       3471`,
        template: `<group name="bgp_neighbors">
BGP neighbor is {{ neighbor_ip }},\s+remote AS {{ remote_as }},\s+{{ link_type }}\s+link
\s+BGP version {{ bgp_version }},\s+remote router ID {{ router_id }}
\s+BGP state = {{ state }},\s+up for {{ uptime }}
</group>`
    },
    
    simple_key_value: {
        name: 'Simple Key-Value Pairs',
        description: 'Parse simple key-value configuration pairs',
        data: `name: router-01
location: datacenter-1
role: core
vendor: cisco
model: ASR9000
version: 7.3.2`,
        template: `<group name="config">
{{ key }}:\s+{{ value }}
</group>`
    },
    
    cable_modem: {
        name: 'Cable Modem Status',
        description: 'Parse cable modem status table (tests variable whitespace and group name with wildcard)',
        data: `centaur-vccap#scm 

MAC Address    IP Address      US             DS           MAC         Prim RxPwr  Timing Num  BPI RPHY MAC

                               Intf           Intf         Status      Sid  (dBmv) Offset CPEs Enb Node Dom

0007.1122.7a73 0.0.0.0         7:0/1.2/0      7:0/0/30     offline     0    0.0    0      0    no  7    7  

001d.d370.4ff2 10.4.13.20      31:0/0.0/0*    31:0/0/24*   online      6277 19.7   1656   1    no  31   31 

001d.d6ca.9b92 10.4.21.232     60:0/1.3/0*    60:0/0/21*   online      5224 20.0   1678   1    no  60   60 `,
        template: `<group name="show.cable.modem*">
{{mac-address}} {{ip-address}}         {{us-intf}}      {{ds-intf}}     {{status}}     {{prim-sid}}    {{rx-power}}    {{timing-offset}}      {{num-cpes}}    {{bpi-enabled}}  {{rphy-node}}    {{mac-domain}} 
</group>`
    }
};

/**
 * Get all example names
 */
function getExampleNames() {
    return Object.keys(TTP_EXAMPLES);
}

/**
 * Get example by name
 */
function getExample(name) {
    return TTP_EXAMPLES[name] || null;
}

/**
 * Get all examples
 */
function getAllExamples() {
    return TTP_EXAMPLES;
}

