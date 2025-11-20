package formatters

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// N2GFormatter formats results as network diagram XML (GraphML or draw.io)
type N2GFormatter struct{}

// NewN2GFormatter creates a new N2G formatter
func NewN2GFormatter() *N2GFormatter {
	return &N2GFormatter{}
}

// N2GOptions contains options for N2G formatting
type N2GOptions struct {
	Module      string                 // "yed" or "drawio"
	Path        string                 // Dot-separated path to results data
	Method      string                 // "from_list", "from_dict", "from_csv"
	MethodKwargs map[string]interface{} // Keyword arguments for method
	NodeDups    string                 // "skip", "log", "update"
	LinkDups    string                 // "skip", "log", "update"
	Algo        string                 // Layout algorithm name
}

// Format formats data as N2G diagram XML
func (f *N2GFormatter) Format(data interface{}, options *N2GOptions) ([]byte, error) {
	if options == nil {
		options = &N2GOptions{
			Module:   "yed",
			Method:   "from_list",
			NodeDups: "skip",
			LinkDups: "skip",
		}
	}

	// Normalize data to list
	var dataList []interface{}
	switch v := data.(type) {
	case []interface{}:
		dataList = v
	case map[string]interface{}:
		dataList = []interface{}{v}
	default:
		return nil, fmt.Errorf("unsupported data type for N2G formatter")
	}

	// Extract data based on path if specified
	var extractedData []interface{}
	if options.Path != "" {
		// TODO: Implement path traversal
		// For now, use data as-is
		extractedData = dataList
	} else {
		extractedData = dataList
	}

	// Build graph from extracted data
	graph := &Graph{
		Nodes: make(map[string]*Node),
		Links: []*Link{},
	}

	// Process data based on method
	switch options.Method {
	case "from_list":
		if err := f.buildGraphFromList(extractedData, graph, options); err != nil {
			return nil, err
		}
	case "from_dict":
		if err := f.buildGraphFromDict(extractedData, graph, options); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported method: %s", options.Method)
	}

	// Apply layout algorithm if specified
	if options.Algo != "" {
		f.applyLayout(graph, options.Algo)
	}

	// Generate XML based on module
	switch strings.ToLower(options.Module) {
	case "yed":
		return f.generateGraphML(graph)
	case "drawio", "draw.io":
		return f.generateDrawIO(graph)
	default:
		return nil, fmt.Errorf("unsupported module: %s (supported: yed, drawio)", options.Module)
	}
}

// Graph represents a network graph with nodes and links
type Graph struct {
	Nodes map[string]*Node
	Links []*Link
}

// Node represents a graph node
type Node struct {
	ID          string
	Label       string
	TopLabel    string
	BottomLabel string
	X           float64
	Y           float64
	Properties  map[string]interface{}
}

// Link represents a graph link/edge
type Link struct {
	Source      string
	Target      string
	SourceLabel string
	TargetLabel string
	Properties  map[string]interface{}
}

// buildGraphFromList builds graph from list of dictionaries
func (f *N2GFormatter) buildGraphFromList(data []interface{}, graph *Graph, options *N2GOptions) error {
	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract node information
		// Look for common N2G fields: id, source, target, label, etc.
		var sourceID, targetID, sourceLabel, targetLabel, label string

		// Try to extract from common field names
		if id, ok := itemMap["id"].(string); ok {
			sourceID = id
		} else if id, ok := itemMap["source"].(string); ok {
			sourceID = id
		} else if id, ok := itemMap["hostname"].(string); ok {
			sourceID = id
		}

		if id, ok := itemMap["target"].(string); ok {
			targetID = id
		} else if id, ok := itemMap["target.id"].(string); ok {
			targetID = id
		}

		if l, ok := itemMap["label"].(string); ok {
			label = l
		} else if l, ok := itemMap["target.top_label"].(string); ok {
			label = l
		}

		if l, ok := itemMap["src_label"].(string); ok {
			sourceLabel = l
		} else if l, ok := itemMap["source_label"].(string); ok {
			sourceLabel = l
		}

		if l, ok := itemMap["trgt_label"].(string); ok {
			targetLabel = l
		} else if l, ok := itemMap["target_label"].(string); ok {
			targetLabel = l
		}

		// Add source node if it exists
		if sourceID != "" {
			if _, exists := graph.Nodes[sourceID]; !exists || options.NodeDups == "update" {
				node := &Node{
					ID:         sourceID,
					Label:      sourceID,
					Properties: make(map[string]interface{}),
				}
				if label != "" {
					node.Label = label
				}
				graph.Nodes[sourceID] = node
			}
		}

		// Add target node if it exists
		if targetID != "" {
			if _, exists := graph.Nodes[targetID]; !exists || options.NodeDups == "update" {
				node := &Node{
					ID:         targetID,
					Label:      targetID,
					Properties: make(map[string]interface{}),
				}
				if topLabel, ok := itemMap["target.top_label"].(string); ok {
					node.TopLabel = topLabel
				}
				if bottomLabel, ok := itemMap["target.bottom_label"].(string); ok {
					node.BottomLabel = bottomLabel
				}
				graph.Nodes[targetID] = node
			}
		}

		// Add link if both source and target exist
		if sourceID != "" && targetID != "" {
			link := &Link{
				Source:      sourceID,
				Target:      targetID,
				SourceLabel: sourceLabel,
				TargetLabel: targetLabel,
				Properties:  make(map[string]interface{}),
			}
			graph.Links = append(graph.Links, link)
		}
	}

	return nil
}

// buildGraphFromDict builds graph from dictionary structure
func (f *N2GFormatter) buildGraphFromDict(data []interface{}, graph *Graph, options *N2GOptions) error {
	// Similar to from_list but expects different structure
	// For now, treat it the same way
	return f.buildGraphFromList(data, graph, options)
}

// applyLayout applies layout algorithm to graph
func (f *N2GFormatter) applyLayout(graph *Graph, algo string) {
	// Basic layout: simple grid or force-directed-like positioning
	// For now, implement a simple grid layout
	nodeList := make([]*Node, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeList = append(nodeList, node)
	}

	switch algo {
	case "grid":
		f.gridLayout(nodeList)
	case "kk", "kamada-kawai":
		// Simplified Kamada-Kawai-like layout
		f.simpleForceLayout(nodeList, graph.Links)
	default:
		// Default: simple grid
		f.gridLayout(nodeList)
	}
}

// gridLayout arranges nodes in a grid
func (f *N2GFormatter) gridLayout(nodes []*Node) {
	cols := int(float64(len(nodes))*0.5) + 1
	if cols < 1 {
		cols = 1
	}
	spacing := 200.0

	for i, node := range nodes {
		row := i / cols
		col := i % cols
		node.X = float64(col) * spacing
		node.Y = float64(row) * spacing
	}
}

// simpleForceLayout applies a simple force-directed layout
func (f *N2GFormatter) simpleForceLayout(nodes []*Node, links []*Link) {
	// Very simplified force-directed layout
	// Start with grid
	f.gridLayout(nodes)

	// Simple iterations to improve layout
	for iter := 0; iter < 10; iter++ {
		for _, link := range links {
			sourceNode := nodes[0]
			targetNode := nodes[0]
			for _, n := range nodes {
				if n.ID == link.Source {
					sourceNode = n
				}
				if n.ID == link.Target {
					targetNode = n
				}
			}

			// Simple spring force
			dx := targetNode.X - sourceNode.X
			dy := targetNode.Y - sourceNode.Y
			dist := dx*dx + dy*dy
			if dist > 0 {
				force := 0.1
				sourceNode.X += dx * force * 0.01
				sourceNode.Y += dy * force * 0.01
				targetNode.X -= dx * force * 0.01
				targetNode.Y -= dy * force * 0.01
			}
		}
	}
}

// GraphML types for XML generation
type graphML struct {
	XMLName xml.Name      `xml:"graphml"`
	Xmlns   string        `xml:"xmlns,attr"`
	XmlnsY  string        `xml:"xmlns:y,attr"`
	XmlnsYE string        `xml:"xmlns:yed,attr"`
	Key     []graphMLKey  `xml:"key"`
	Graph   graphMLGraph  `xml:"graph"`
}

type graphMLKey struct {
	ID   string `xml:"id,attr"`
	For  string `xml:"for,attr"`
	Name string `xml:"attr.name,attr"`
	Type string `xml:"attr.type,attr"`
}

type graphMLGraph struct {
	XMLName     xml.Name       `xml:"graph"`
	ID          string         `xml:"id,attr"`
	Edgedefault string         `xml:"edgedefault,attr"`
	Node        []graphMLNode  `xml:"node"`
	Edge        []graphMLEdge  `xml:"edge"`
}

type graphMLNode struct {
	ID   string        `xml:"id,attr"`
	Data []graphMLData `xml:"data"`
}

type graphMLEdge struct {
	ID     string        `xml:"id,attr"`
	Source string        `xml:"source,attr"`
	Target string        `xml:"target,attr"`
	Data   []graphMLData `xml:"data"`
}

type graphMLData struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",innerxml"`
}

// generateGraphML generates GraphML XML for yEd
func (f *N2GFormatter) generateGraphML(graph *Graph) ([]byte, error) {

	graphml := graphML{
		Xmlns:   "http://graphml.graphdrawing.org/xmlns",
		XmlnsY:  "http://www.yworks.com/xml/graphml",
		XmlnsYE: "http://www.yworks.com/xml/yed/3",
		Key: []graphMLKey{
			{ID: "d0", For: "node", Name: "description", Type: "string"},
			{ID: "d1", For: "node", Name: "url", Type: "string"},
			{ID: "d2", For: "node", Name: "description", Type: "string"},
			{ID: "d3", For: "edge", Name: "description", Type: "string"},
			{ID: "d4", For: "node", Name: "CustomGraphics", Type: "y:ShapeNode"},
			{ID: "d5", For: "edge", Name: "CustomGraphics", Type: "y:PolyLineEdge"},
		},
	}

	// Convert nodes
	nodes := make([]graphMLNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		gnode := graphMLNode{
			ID: node.ID,
			Data: []graphMLData{
				{Key: "d4", Value: fmt.Sprintf(`<y:ShapeNode>
					<y:Geometry x="%f" y="%f" width="100" height="50"/>
					<y:Fill color="#FFCC00" transparent="false"/>
					<y:BorderStyle color="#000000" type="line" width="1.0"/>
					<y:NodeLabel>%s</y:NodeLabel>
				</y:ShapeNode>`, node.X, node.Y, node.Label)},
			},
		}
		nodes = append(nodes, gnode)
	}

	// Convert edges
	edges := make([]graphMLEdge, 0, len(graph.Links))
	for i, link := range graph.Links {
		edge := graphMLEdge{
			ID:     fmt.Sprintf("e%d", i),
			Source: link.Source,
			Target: link.Target,
			Data: []graphMLData{
				{Key: "d5", Value: `<y:PolyLineEdge>
					<y:Path sx="0.0" sy="0.0" tx="0.0" ty="0.0"/>
					<y:LineStyle color="#000000" type="line" width="1.0"/>
					<y:Arrows source="none" target="standard"/>
					<y:BendStyle smoothed="false"/>
				</y:PolyLineEdge>`},
			},
		}
		edges = append(edges, edge)
	}

	graphml.Graph = graphMLGraph{
		ID:          "G",
		Edgedefault: "directed",
		Node:        nodes,
		Edge:        edges,
	}

	return xml.MarshalIndent(graphml, "", "  ")
}

// Draw.io types for XML generation
type mxGraphModel struct {
	XMLName    xml.Name `xml:"mxGraphModel"`
	DX         string   `xml:"dx,attr"`
	DY         string   `xml:"dy,attr"`
	Grid       string   `xml:"grid,attr"`
	GridSize   string   `xml:"gridSize,attr"`
	Guides     string   `xml:"guides,attr"`
	Tooltips   string   `xml:"tooltips,attr"`
	Connect    string   `xml:"connect,attr"`
	Arrows     string   `xml:"arrows,attr"`
	Fold       string   `xml:"fold,attr"`
	Page       string   `xml:"page,attr"`
	PageScale  string   `xml:"pageScale,attr"`
	PageWidth  string   `xml:"pageWidth,attr"`
	PageHeight string   `xml:"pageHeight,attr"`
	Math       string   `xml:"math,attr"`
	Shadow     string   `xml:"shadow,attr"`
	Root       mxRoot   `xml:"root"`
}

type mxRoot struct {
	MxCell []mxCell `xml:"mxCell"`
}

type mxCell struct {
	ID       string      `xml:"id,attr"`
	Value    string      `xml:"value,attr,omitempty"`
	Style    string      `xml:"style,attr,omitempty"`
	Parent   string      `xml:"parent,attr,omitempty"`
	Source   string      `xml:"source,attr,omitempty"`
	Target   string      `xml:"target,attr,omitempty"`
	Edge     string      `xml:"edge,attr,omitempty"`
	Vertex   string      `xml:"vertex,attr,omitempty"`
	Geometry *mxGeometry `xml:"mxGeometry,omitempty"`
}

type mxGeometry struct {
	X      string `xml:"x,attr,omitempty"`
	Y      string `xml:"y,attr,omitempty"`
	Width  string `xml:"width,attr,omitempty"`
	Height string `xml:"height,attr,omitempty"`
	As     string `xml:"as,attr"`
}

// generateDrawIO generates draw.io XML format
func (f *N2GFormatter) generateDrawIO(graph *Graph) ([]byte, error) {

	model := mxGraphModel{
		DX:         "1422",
		DY:         "794",
		Grid:       "1",
		GridSize:   "10",
		Guides:     "1",
		Tooltips:   "1",
		Connect:    "1",
		Arrows:     "1",
		Fold:       "1",
		Page:       "1",
		PageScale:  "1",
		PageWidth:  "1169",
		PageHeight: "827",
		Math:       "0",
		Shadow:     "0",
	}

	// Root cell
	root := mxRoot{
		MxCell: []mxCell{
			{ID: "0"},
			{ID: "1", Parent: "0"},
		},
	}

	// Add nodes
	nodeIDMap := make(map[string]string)
	nodeCounter := 2
	for _, node := range graph.Nodes {
		nodeID := fmt.Sprintf("%d", nodeCounter)
		nodeIDMap[node.ID] = nodeID
		nodeCounter++

		cell := mxCell{
			ID:     nodeID,
			Value:  node.Label,
			Style:  "rounded=1;whiteSpace=wrap;html=1;",
			Parent: "1",
			Vertex: "1",
			Geometry: &mxGeometry{
				X:      fmt.Sprintf("%.0f", node.X),
				Y:      fmt.Sprintf("%.0f", node.Y),
				Width:  "120",
				Height: "60",
				As:     "geometry",
			},
		}
		root.MxCell = append(root.MxCell, cell)
	}

	// Add edges
	for _, link := range graph.Links {
		sourceID, ok1 := nodeIDMap[link.Source]
		targetID, ok2 := nodeIDMap[link.Target]
		if !ok1 || !ok2 {
			continue
		}

		cell := mxCell{
			ID:     fmt.Sprintf("%d", nodeCounter),
			Parent: "1",
			Source: sourceID,
			Target: targetID,
			Edge:   "1",
			Geometry: &mxGeometry{
				As: "geometry",
			},
		}
		root.MxCell = append(root.MxCell, cell)
		nodeCounter++
	}

	model.Root = root

	return xml.MarshalIndent(model, "", "  ")
}

// FormatString formats data as N2G diagram XML string
func (f *N2GFormatter) FormatString(data interface{}, options *N2GOptions) (string, error) {
	bytes, err := f.Format(data, options)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

