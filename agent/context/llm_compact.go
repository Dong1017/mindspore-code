package context

import (
	stdctx "context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
)

const (
	envCompactMode       = "MSCLI_COMPACT_MODE"
	envDisableLLMCompact = "MSCLI_DISABLE_LLM_COMPACT"

	compactModeLLM      = "llm"
	compactModeLegacy   = "legacy"
	compactModePriority = "priority"
)

const compactSummarySystemPrompt = "You are a helpful AI assistant tasked with summarizing conversations."

const compactSummaryPrompt = `
CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: ensure that this step is DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary request. If your last task was concluded, then only list next steps if they are explicitly in line with the users request. Do not start on tangential requests or really old requests that were already completed without confirming with the user first.
                       If there is a next step, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no drift in task interpretation.

Here's an example of how your output should be structured:

<example>
<analysis>
[Your thought process, ensuring all points are covered thoroughly and accurately]
</analysis>

<summary>
1. Primary Request and Intent:
   [Detailed description]

2. Key Technical Concepts:
   - [Concept 1]
   - [Concept 2]
   - [...]

3. Files and Code Sections:
   - [File Name 1]
      - [Summary of why this file is important]
      - [Summary of the changes made to this file, if any]
      - [Important Code Snippet]
   - [File Name 2]
      - [Important Code Snippet]
   - [...]

4. Errors and fixes:
    - [Detailed description of error 1]:
      - [How you fixed the error]
      - [User feedback on the error if any]
    - [...]

5. Problem Solving:
   [Description of solved problems and ongoing troubleshooting]

6. All user messages:` + " " + `
    - [Detailed non tool use user message]
    - [...]

7. Pending Tasks:
   - [Task 1]
   - [Task 2]
   - [...]

8. Current Work:
   [Precise description of current work]

9. Optional Next Step:
   [Optional Next step to take]

</summary>
</example>

Please provide your summary based on the conversation so far, following this structure and ensuring precision and thoroughness in your response.` + " " + `

There may be additional summarization instructions provided in the included context. If so, remember to follow these instructions when creating the above summary. Examples of instructions include:
<example>
## Compact Instructions
When summarizing the conversation focus on typescript code changes and also remember the mistakes you made and how you fixed them.
</example>

<example>
# Summary instructions
When you are using compact - please focus on test output and code changes. Include file reads verbatim.
</example>


REMINDER: Do NOT call any tools. Respond with plain text only — an <analysis> block followed by a <summary> block. Tool calls will be rejected and you will fail the task.`

const rewindSummaryPrompt = `
CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have the recent conversation segment in the messages above.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

Your task is to summarize the RECENT portion of the conversation that is about to be removed by a rewind. Earlier messages are being kept intact and do not need to be summarized. Focus only on what happened in the recent messages: user requests, decisions, files or commands discussed, changes made, errors and fixes, current state, and any pending follow-up that remains relevant.

The summary must be useful as background context after the conversation is rewound. Do not invent work that was not present in the recent messages.

Respond using this structure:

<analysis>
[Briefly verify the important details to preserve.]
</analysis>

<summary>
1. Primary Request and Intent:
   [Recent user requests and intent]
2. Key Technical Concepts:
   [Important concepts, tools, files, packages, or commands]
3. Files and Code Sections:
   [Files examined or changed, with relevant details]
4. Errors and fixes:
   [Errors encountered and how they were handled]
5. Problem Solving:
   [Problems solved and important decisions]
6. All user messages:
   [Recent non-tool user messages]
7. Pending Tasks:
   [Remaining tasks, if any]
8. Current Work:
   [State immediately before rewind]
9. Optional Next Step:
   [Only if directly implied by the recent work]
</summary>

REMINDER: Do NOT call any tools. Respond with plain text only — an <analysis> block followed by a <summary> block.`

var (
	compactAnalysisBlockRE = regexp.MustCompile(`(?is)<analysis>.*?</analysis>`)
	compactSummaryBlockRE  = regexp.MustCompile(`(?is)<summary>(.*?)</summary>`)
)

func compactModeFromEnv() string {
	if isTruthyEnv(os.Getenv(envDisableLLMCompact)) {
		return compactModeLegacy
	}

	mode := strings.ToLower(strings.TrimSpace(os.Getenv(envCompactMode)))
	switch mode {
	case "", compactModeLLM:
		return compactModeLLM
	case compactModeLegacy, "fallback", "heuristic":
		return compactModeLegacy
	case compactModePriority:
		return compactModePriority
	default:
		return compactModeLLM
	}
}

func isTruthyEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (m *Manager) compactWithLLMLocked(ctx stdctx.Context, targetTokens int) ([]llm.Message, CompactResult, error) {
	if m.provider == nil {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("llm compact provider is not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, err
	}
	if len(m.messages) == 0 {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("not enough messages to compact")
	}

	maxTokens := compactSummaryMaxTokens(targetTokens)
	reqMessages := compactSummaryRequestMessages(m.messages)
	req := &llm.CompletionRequest{
		Messages:  reqMessages,
		MaxTokens: &maxTokens,
	}

	if m.dumper != nil {
		ctx = llm.WithDebugDumper(ctx, m.dumper)
	}
	resp, err := m.provider.Complete(ctx, req)
	if err != nil {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("generate compact summary: %w", err)
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("generate compact summary: empty response")
	}
	if len(resp.ToolCalls) > 0 {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("generate compact summary: model attempted tool use")
	}

	summaryMsg := llm.NewUserMessage(compactContinuationMessage(summary, m.trajectoryPath))
	compacted := []llm.Message{summaryMsg}
	if estimateMessagesWithSystem(compacted, m.system, m.tokenizer) > targetTokens {
		return nil, CompactResult{Strategy: CompactStrategyLLM}, fmt.Errorf("generated compact summary exceeds target budget")
	}

	return compacted, CompactResult{
		Kept:     len(compacted),
		Removed:  len(m.messages) - len(compacted),
		Strategy: CompactStrategyLLM,
		Summary:  formatCompactSummary(summary),
	}, nil
}

func compactSummaryMaxTokens(targetTokens int) int {
	maxTokens := targetTokens / 2
	if maxTokens < 512 {
		maxTokens = 512
	}
	if maxTokens > 20000 {
		maxTokens = 20000
	}
	return maxTokens
}

func compactSummaryRequestMessages(messages []llm.Message) []llm.Message {
	reqMessages := make([]llm.Message, 0, len(messages)+2)
	reqMessages = append(reqMessages, llm.NewSystemMessage(compactSummarySystemPrompt))
	reqMessages = append(reqMessages, messages...)
	reqMessages = append(reqMessages, llm.NewUserMessage(compactSummaryPrompt))
	return reqMessages
}

// SummarizeRewindSegmentWithContext summarizes the segment that will be removed by a rewind.
func (m *Manager) SummarizeRewindSegmentWithContext(ctx stdctx.Context, messages []llm.Message, userContext string) (llm.Message, string, error) {
	if m == nil {
		return llm.Message{}, "", fmt.Errorf("context manager is nil")
	}
	if ctx == nil {
		ctx = stdctx.Background()
	}
	if err := ctx.Err(); err != nil {
		return llm.Message{}, "", err
	}
	if len(messages) == 0 {
		return llm.Message{}, "", fmt.Errorf("nothing to summarize after the selected checkpoint")
	}

	m.mu.RLock()
	provider := m.provider
	dumper := m.dumper
	trajectoryPath := m.trajectoryPath
	targetTokens := m.config.ContextWindow - m.config.ReserveTokens
	m.mu.RUnlock()

	if provider == nil {
		return llm.Message{}, "", fmt.Errorf("llm compact provider is not configured")
	}
	if targetTokens <= 0 {
		targetTokens = compactTargetTokens
	}
	maxTokens := compactSummaryMaxTokens(targetTokens)
	req := &llm.CompletionRequest{
		Messages:  rewindSummaryRequestMessages(messages, userContext),
		MaxTokens: &maxTokens,
	}
	if dumper != nil {
		ctx = llm.WithDebugDumper(ctx, dumper)
	}
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return llm.Message{}, "", fmt.Errorf("generate rewind summary: %w", err)
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return llm.Message{}, "", fmt.Errorf("generate rewind summary: empty response")
	}
	if len(resp.ToolCalls) > 0 {
		return llm.Message{}, "", fmt.Errorf("generate rewind summary: model attempted tool use")
	}

	formatted := formatCompactSummary(summary)
	return llm.NewUserMessage(rewindSummaryContinuationMessage(formatted, trajectoryPath)), formatted, nil
}

func rewindSummaryRequestMessages(messages []llm.Message, userContext string) []llm.Message {
	prompt := rewindSummaryPrompt
	if context := strings.TrimSpace(userContext); context != "" {
		prompt += "\n\nAdditional context from the user:\n" + context
	}
	reqMessages := make([]llm.Message, 0, len(messages)+2)
	reqMessages = append(reqMessages, llm.NewSystemMessage(compactSummarySystemPrompt))
	reqMessages = append(reqMessages, messages...)
	reqMessages = append(reqMessages, llm.NewUserMessage(prompt))
	return reqMessages
}

func formatCompactSummary(summary string) string {
	formatted := compactAnalysisBlockRE.ReplaceAllString(summary, "")
	if match := compactSummaryBlockRE.FindStringSubmatch(formatted); len(match) >= 2 {
		formatted = compactSummaryBlockRE.ReplaceAllString(formatted, "Summary:\n"+strings.TrimSpace(match[1]))
	}
	formatted = strings.ReplaceAll(formatted, "\r\n", "\n")
	for strings.Contains(formatted, "\n\n\n") {
		formatted = strings.ReplaceAll(formatted, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(formatted)
}

func compactContinuationMessage(summary, trajectoryPath string) string {
	var b strings.Builder
	b.WriteString("This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n")
	b.WriteString(formatCompactSummary(summary))
	if path := strings.TrimSpace(trajectoryPath); path != "" {
		b.WriteString("\n\nReference: the full trajectory is available at: ")
		b.WriteString(path)
	}
	b.WriteString("\n\nContinue from where the conversation left off. Do not acknowledge this summary unless the user asks about it.")
	return b.String()
}

func rewindSummaryContinuationMessage(summary, trajectoryPath string) string {
	var b strings.Builder
	b.WriteString("This conversation was rewound. The summary below covers the messages removed by the rewind, from the selected point through the latest state.\n\n")
	b.WriteString(summary)
	if path := strings.TrimSpace(trajectoryPath); path != "" {
		b.WriteString("\n\nReference: the full trajectory is available at: ")
		b.WriteString(path)
	}
	b.WriteString("\n\nUse this as background context only. Do not treat it as a new user request, and do not acknowledge it unless the user asks about it.")
	return b.String()
}
