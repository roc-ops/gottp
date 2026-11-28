package compiler

// ttpFunctionNames are Python TTP 0.10.1's own function tables, the names
// TTP looks a function up in for each scope (ttp_dict_cache.pickle;
// testdata/gen_ttp_functions.py records them in testdata/ttp_functions.json,
// which TestTTPFunctionNames checks this table against). A name here that
// gottp doesn't implement is valid TTP, so the compiler warns that gottp
// doesn't support it rather than calling it unknown.
var ttpFunctionNames = map[Scope][]string{
	ScopeMatch: {
		"append", "cidr_match", "contains", "contains_re", "copy", "count", "dns",
		"endswith_re", "equal", "exclude", "exclude_re", "geoip_lookup",
		"gpvlookup", "greaterthan", "ip_info", "is_ip", "isdigit", "item", "join",
		"joinmatches", "lessthan", "let", "lookup", "mac_eui", "macro", "notdigit",
		"notendswith_re", "notequal", "notstartswith_re", "prepend", "print",
		"raise", "rdns", "record", "replaceall", "resub", "resuball", "rlookup",
		"set", "sformat", "startswith_re", "to_cidr", "to_float", "to_int",
		"to_ip", "to_list", "to_net", "to_str", "to_unicode", "truncate",
		"unrange", "uptimeparse", "void",
	},
	ScopeGroup: {
		"cerberus", "contains", "contains_val", "containsall", "del", "equal",
		"exclude", "exclude_val", "excludeall", "expand", "itemize", "items2dict",
		"lookup", "macro", "record", "set", "sformat", "str_to_unicode", "to_int",
		"to_ip", "validate", "void",
	},
	ScopeInput: {
		"extract_commands", "macro", "test",
	},
	ScopeOutput: {
		"deepdiff", "dict_to_list", "is_equal", "macro", "traverse", "validate",
		"validate_yangson",
	},
}
