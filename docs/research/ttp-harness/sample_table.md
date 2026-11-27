| # | TTP test (file::name) | gottp result | first difference (path: Python → gottp) |
|---|---|---|---|
| 1 | `test_misc.py::test_match_vars_with_hyphen` | pass-normalized | — |
| 2 | `test_anonymous_group.py::test_simple_anonymous_template` | pass-normalized | — |
| 3 | `test_empty_group_results.py::test_empty_group_results` | pass-strict | — |
| 4 | `test_answers_and_docs.py::test_github_issue_22` | fail | `$.len`: `2` → `0` |
| 5 | `test_match_condition_fun.py::test_contains_re_from_vars` | fail | `$[0].is_lag`: `true` → `"True"` |
| 6 | `test_match_condition_fun.py::test_startswith_re_inline` | fail | `$.len`: `1` → `0` |
| 7 | `test_match_condition_fun.py::test_exclude_inline` | fail | `$[0].description`: `"<missing>"` → `"RID"` |
| 8 | `test_match_functions.py::test_match_variable_default_set_to_string` | fail | `$[0].ip`: `"1.1.1.1/24"` → `" 1.1.1.1/24"` |
| 9 | `test_match_functions.py::test_let_and_set` | fail | `$[0].admin-status`: `"down"` → `"up"` |
| 10 | `test_match_functions.py::test_copy` | fail | `$[0].ip_address`: `"1.1.1.123"` → `"<missing>"` |
| 11 | `test_headers_indicator.py::test_headers_docs_example` | fail | `$.len`: `3` → `0` |
| 12 | `test_extend_tag.py::test_extend_tag_within_group` | fail | `$[0].bgp_config.config`: `{"rid": "1.1.1.1"}` → `"<missing>"` |
| 13 | `test_group_lookup.py::test_group_lookup_using_lookup_table_action_replace` | fail | `$[0].arp[0].ip`: `"app_1"` → `"10.12.13.2"` |
| 14 | `test_group_functions.py::test_group_to_int_missing_key_issue_109` | fail | `$[2].mtu`: `9200` → `"9200.0"` |
| 15 | `test_output_formatters.py::test_csv_formatter_simple` | fail | `$[0]`: `"\"interface\",\"vlan\"\n\"Port-Chanel11\",\"10\"\n\"Loopbac` → `"\"interface\",\"vlan\"\n\"Port-Chanel11\",\"10\""` |
| 16 | `test_structure.py::test_list_structure` | fail | `$[0]`: `[{"ip": "192.168.0.1", "mask": "32", "interface": "Lo0"}, {"` → `{"interface": "Lo0", "ip": "192.168.0.1", "mask": "32"}` |
| 17 | `test_misc.py::test_asterisk_issue_97` | fail | `$[0].interfaces.lag-4:1629`: `"<missing>"` → `{"*": {"interface": "port lag-4:1629"}}` |
| 18 | `test_answers_and_docs.py::test_vlans_parsing` | fail | `$[0].vlans_active`: `{"interfaces": [{"port": "Gi0", "vlans": ["1", "8", "999"]},` → `"<missing>"` |
| 19 | `test_match_functions.py::test_multiple_joinmatches` | fail | `$[0].vlans[0].Hostname`: `"SWITCH"` → `"gethostname"` |
| 20 | `test_answers_and_docs.py::test_issue_50` | fail | `$[0].ifaces.BNG-RH201-CORE.description`: `"BNG-RH201-CORE"` → `"\"BNG-RH201-CORE\""` |
| 21 | `test_N2G_formatter.py::test_n2g_formatter` | error | gottp error: `parse: failed to parse with template: failed to process output functions: unknown output format: n2g` |
| 22 | `test_group_functions.py::test_group_macro_chaining` | error | gottp error: `parse: failed to parse with template: failed to parse group : failed to process group macro: group macro [chec` |
