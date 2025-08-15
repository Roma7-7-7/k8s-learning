// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TextProcessingJobSpec defines the desired state of TextProcessingJob
type TextProcessingJobSpec struct {
	// ProcessingType specifies the type of text processing to perform
	// +kubebuilder:validation:Enum=wordcount;linecount;uppercase;lowercase;replace;extract
	ProcessingType string `json:"processingType"`

	// Parameters contains processing-specific parameters as JSON
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Parameters map[string]string `json:"parameters,omitempty"`

	// Priority specifies the job priority (1-10, with 10 being highest)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=5
	Priority int32 `json:"priority,omitempty"`

	// DelayMs specifies an artificial delay in milliseconds for stress testing
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=60000
	// +optional
	DelayMs int32 `json:"delayMs,omitempty"`

	// Replicas specifies the desired number of worker pods for this job type
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`
}

// TextProcessingJobStatus defines the observed state of TextProcessingJob
type TextProcessingJobStatus struct {
	// Phase represents the current phase of the job
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// StartTime is when the job started processing
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job finished
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message provides additional information about the job status
	// +optional
	Message string `json:"message,omitempty"`

	// QueueDepth shows the current queue depth for this job type
	// +optional
	QueueDepth int32 `json:"queueDepth,omitempty"`

	// ActiveReplicas shows the current number of active worker replicas
	// +optional
	ActiveReplicas int32 `json:"activeReplicas,omitempty"`

	// ProcessedJobs shows the total number of jobs processed
	// +optional
	ProcessedJobs int32 `json:"processedJobs,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.activeReplicas
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Processing Type",type="string",JSONPath=".spec.processingType"
// +kubebuilder:printcolumn:name="Priority",type="integer",JSONPath=".spec.priority"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Queue Depth",type="integer",JSONPath=".status.queueDepth"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TextProcessingJob is the Schema for the textprocessingjobs API
type TextProcessingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TextProcessingJobSpec   `json:"spec,omitempty"`
	Status TextProcessingJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TextProcessingJobList contains a list of TextProcessingJob
type TextProcessingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TextProcessingJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TextProcessingJob{}, &TextProcessingJobList{})
}