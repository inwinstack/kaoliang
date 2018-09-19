package models

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/minio/minio/pkg/event"
	"github.com/minio/minio/pkg/wildcard"
)

type Model struct {
	ID        uint       `gorm:"primary_key" xml:"-"`
	CreatedAt time.Time  `xml:"-"`
	UpdatedAt time.Time  `xml:"-"`
	DeletedAt *time.Time `xml:"-" sql:"index"`
}

type Event struct {
	Model
	Name    event.Name
	QueueID uint `xml:"-"`
	TopicID uint `xml:"-"`
}

func (e *Event) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}

	eventName, err := event.ParseName(s)
	if err != nil {
		return err
	}

	e.Name = eventName
	return nil
}

type FilterRule struct {
	Model
	Name             string `xml:"Name"`
	Value            string `xml:"Value"`
	FilterRuleListID uint   `xml:"-"`
}

// UnmarshalXML - decodes XML data.
func (filter *FilterRule) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Make subtype to avoid recursive UnmarshalXML().
	type filterRule FilterRule
	rule := filterRule{}
	if err := d.DecodeElement(&rule, &start); err != nil {
		return err
	}

	*filter = FilterRule(rule)

	return nil
}

type FilterRuleList struct {
	Model
	Rules   []FilterRule `xml:"FilterRule,omitempty"`
	S3KeyID uint
}

// UnmarshalXML - decodes XML data.
func (ruleList *FilterRuleList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Make subtype to avoid recursive UnmarshalXML().
	type filterRuleList FilterRuleList
	rules := filterRuleList{}
	if err := d.DecodeElement(&rules, &start); err != nil {
		return err
	}

	*ruleList = FilterRuleList(rules)
	return nil
}

// Pattern - returns pattern using prefix and suffix values.
func (ruleList FilterRuleList) Pattern() string {
	var prefix string
	var suffix string

	for _, rule := range ruleList.Rules {
		switch rule.Name {
		case "prefix":
			prefix = rule.Value
		case "suffix":
			suffix = rule.Value
		}
	}

	return NewPattern(prefix, suffix)
}

// NewPattern - create new pattern for prefix/suffix.
func NewPattern(prefix, suffix string) (pattern string) {
	if prefix != "" {
		if !strings.HasSuffix(prefix, "*") {
			prefix += "*"
		}

		pattern = prefix
	}

	if suffix != "" {
		if !strings.HasPrefix(suffix, "*") {
			suffix = "*" + suffix
		}

		pattern += suffix
	}

	pattern = strings.Replace(pattern, "**", "*", -1)

	return pattern
}

type S3Key struct {
	Model
	RuleList FilterRuleList `xml:"S3Key,omitempty"`
	QueueID  uint
	TopicID  uint
}

func (q Queue) ToRulesMap() RulesMap {
	pattern := q.Filter.RuleList.Pattern()

	names := make([]event.Name, len(q.Events))

	for _, e := range q.Events {
		names = append(names, e.Name)
	}

	return NewRulesMap(names, pattern, q.Resource)
}

type Queue struct {
	Model
	QueueIdentifier string   `xml:"Id" gorm:"unique;not null"`
	Filter          S3Key    `xml:"Filter"`
	Events          []Event  `xml:"Event"`
	ARN             string   `xml:"Queue"`
	Resource        Resource `xml:"-"`
	ResourceID      uint     `xml:"-"`
	ConfigID        uint     `xml:"-"`
}

// UnmarshalXML - decodes XML data.
func (q *Queue) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Make subtype to avoid recursive UnmarshalXML().
	type queue Queue
	parsedQueue := queue{}
	if err := d.DecodeElement(&parsedQueue, &start); err != nil {
		return err
	}

	*q = Queue(parsedQueue)

	return nil
}

type Topic struct {
	Model
	TopicIdentifier string   `xml:"Id" gorm:"unique;not null"`
	Filter          S3Key    `xml:"Filter"`
	Events          []Event  `xml:"Event"`
	ARN             string   `xml:"Topic"`
	Resource        Resource `xml:"-"`
	ResourceID      uint     `xml:"-"`
	ConfigID        uint     `xml:"-"`
}

func (t Topic) ToRulesMap() RulesMap {
	pattern := t.Filter.RuleList.Pattern()

	names := make([]event.Name, len(t.Events))

	for _, e := range t.Events {
		names = append(names, e.Name)
	}

	return NewRulesMap(names, pattern, t.Resource)
}

type Config struct {
	Model
	Bucket  string   `xml:"-" gorm:"unique;not null"`
	XMLName xml.Name `xml:"NotificationConfiguration"`
	Queues  []Queue  `xml:"QueueConfiguration,omitempty"`
	Topics  []Topic  `xml:"TopicConfiguration,omitempty"`
}

// UnmarshalXML - decodes XML data.
func (conf *Config) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Make subtype to avoid recursive UnmarshalXML().
	type config Config
	parsedConfig := config{}
	if err := d.DecodeElement(&parsedConfig, &start); err != nil {
		return err
	}

	*conf = Config(parsedConfig)

	return nil
}

func (conf Config) ToRulesMap() RulesMap {
	rulesMap := make(RulesMap)

	for _, queue := range conf.Queues {
		rulesMap.Add(queue.ToRulesMap())
	}
	for _, topic := range conf.Topics {
		rulesMap.Add(topic.ToRulesMap())
	}

	return rulesMap
}

type Rules map[string][]Resource

// Match - returns []Resource matching object name in rules.
func (rules Rules) Match(objectName string) []Resource {
	var matched []Resource

	for pattern, resources := range rules {
		if wildcard.MatchSimple(pattern, objectName) {
			for _, resource := range resources {
				matched = append(matched, resource)
			}
		}
	}

	return matched
}

// Clone - returns copy of this rules.
func (rules Rules) Clone() Rules {
	rulesCopy := make(Rules)

	for pattern, resource := range rules {
		rulesCopy[pattern] = resource
	}

	return rulesCopy
}

// Union - returns union with given rules as new rules.
func (rules Rules) Union(rules2 Rules) Rules {
	nrules := rules.Clone()

	for pattern, resources := range rules2 {
		for _, resource := range resources {
			nrules[pattern] = append(nrules[pattern], resource)
		}
	}

	return nrules
}

type RulesMap map[event.Name]Rules

// add - adds event names, prefixes, suffixes and resource to rules map.
func (rulesMap RulesMap) add(eventNames []event.Name, pattern string, resource Resource) {
	rules := make(Rules)
	rules[pattern] = append(rules[pattern], resource)

	for _, eventName := range eventNames {
		for _, name := range eventName.Expand() {
			rulesMap[name] = rulesMap[name].Union(rules)
		}
	}
}

// Add - adds given rules map.
func (rulesMap RulesMap) Add(rulesMap2 RulesMap) {
	for eventName, rules := range rulesMap2 {
		rulesMap[eventName] = rules.Union(rulesMap[eventName])
	}
}

// NewRulesMap - creates new rules map with given values.
func NewRulesMap(eventNames []event.Name, pattern string, resource Resource) RulesMap {
	// If pattern is empty, add '*' wildcard to match all.
	if pattern == "" {
		pattern = "*"
	}

	rulesMap := make(RulesMap)
	rulesMap.add(eventNames, pattern, resource)
	return rulesMap
}
