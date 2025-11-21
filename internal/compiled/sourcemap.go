package compiled

// SourceMap tracks which parts of input text matched which template patterns
type SourceMap struct {
	Inputs map[string]*InputSourceMap // input name -> source map
}

// InputSourceMap contains source mapping for a single input
type InputSourceMap struct {
	Lines []*LineMapping // one entry per input line
}

// LineMapping represents the mapping for a single line of input
type LineMapping struct {
	LineNumber int            // 0-indexed line number
	Matched    bool           // whether this line matched
	Matches    []*MatchMapping // matches on this line
}

// MatchMapping represents a single match on a line
type MatchMapping struct {
	StartCol     int                  // start column (0-indexed)
	EndCol       int                  // end column (exclusive)
	GroupName    string               // group name that matched
	PatternIndex int                  // pattern index within group
	Variables    map[string]*VarRange // variable name -> character range
	ResultPath   string               // path in result structure (e.g., "interfaces[0]")
}

// VarRange represents the character range for a variable within a match
type VarRange struct {
	StartCol int // start column (0-indexed)
	EndCol   int // end column (exclusive)
}

