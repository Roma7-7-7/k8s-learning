package worker

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rsav/k8s-learning/internal/storage/database"
)

type TextProcessor struct {
	resultDir string
	log       *slog.Logger
}

func NewTextProcessor(resultDir string, logger *slog.Logger) *TextProcessor {
	return &TextProcessor{
		resultDir: resultDir,
		log:       logger,
	}
}

func (tp *TextProcessor) CanProcess(processingType database.ProcessingType) bool {
	switch processingType {
	case database.ProcessingTypeWordCount, database.ProcessingTypeLineCount,
		database.ProcessingTypeUppercase, database.ProcessingTypeLowercase,
		database.ProcessingTypeReplace, database.ProcessingTypeExtract:
		return true
	default:
		return false
	}
}

func (tp *TextProcessor) Process(ctx context.Context, job *ProcessingJob) (string, error) {
	tp.log.InfoContext(ctx, "processing text job",
		"job_id", job.JobID,
		"processing_type", job.ProcessingType,
		"file_path", job.FilePath)

	switch job.ProcessingType {
	case database.ProcessingTypeWordCount:
		return tp.processWordCount(ctx, job)
	case database.ProcessingTypeLineCount:
		return tp.processLineCount(ctx, job)
	case database.ProcessingTypeUppercase:
		return tp.processUppercase(ctx, job)
	case database.ProcessingTypeLowercase:
		return tp.processLowercase(ctx, job)
	case database.ProcessingTypeReplace:
		return tp.processReplace(ctx, job)
	case database.ProcessingTypeExtract:
		return tp.processExtract(ctx, job)
	default:
		return "", NewProcessingLogicError(string(job.ProcessingType), "unsupported processing type")
	}
}

func (tp *TextProcessor) processWordCount(_ context.Context, job *ProcessingJob) (string, error) {
	content, err := tp.readFile(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}

	words := strings.Fields(content)
	result := strconv.Itoa(len(words))

	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) processLineCount(_ context.Context, job *ProcessingJob) (string, error) {
	file, err := os.Open(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return "", NewFileReadError(job.FilePath, fmt.Errorf("scan file: %w", err))
	}

	result := strconv.Itoa(lineCount)
	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) processUppercase(_ context.Context, job *ProcessingJob) (string, error) {
	content, err := tp.readFile(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}

	result := strings.ToUpper(content)
	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) processLowercase(_ context.Context, job *ProcessingJob) (string, error) {
	content, err := tp.readFile(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}

	result := strings.ToLower(content)
	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) processReplace(_ context.Context, job *ProcessingJob) (string, error) {
	find, ok := job.Parameters["find"].(string)
	if !ok || find == "" {
		return "", NewInvalidParamError("find", "missing or empty")
	}

	replaceWith, ok := job.Parameters["replace_with"].(string)
	if !ok {
		return "", NewInvalidParamError("replace_with", "missing or not a string")
	}

	content, err := tp.readFile(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}

	result := strings.ReplaceAll(content, find, replaceWith)
	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) processExtract(_ context.Context, job *ProcessingJob) (string, error) {
	pattern, ok := job.Parameters["pattern"].(string)
	if !ok || pattern == "" {
		return "", NewInvalidParamError("pattern", "missing or empty")
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", NewRegexCompileError(pattern, err)
	}

	content, err := tp.readFile(job.FilePath)
	if err != nil {
		return "", NewFileReadError(job.FilePath, err)
	}

	matches := regex.FindAllString(content, -1)
	result := strings.Join(matches, "\n")

	outputPath, err := tp.writeResult(job.JobID, result)
	if err != nil {
		return "", NewFileWriteError(outputPath, err)
	}

	return outputPath, nil
}

func (tp *TextProcessor) readFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(content), nil
}

func (tp *TextProcessor) writeResult(jobID, content string) (string, error) {
	filename := fmt.Sprintf("result_%s.txt", jobID)
	outputPath := filepath.Join(tp.resultDir, filename)

	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write result file: %w", err)
	}

	return outputPath, nil
}
