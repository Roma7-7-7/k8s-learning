// API Base URL - auto-detect environment
function getApiBaseUrl() {
    // Check if we're running via file:// protocol (local development)
    if (window.location.protocol === 'file:') {
        return 'http://localhost:8080/api';
    }
    
    // Check if we're running on localhost with a port (local development)
    const isLocalDev = window.location.hostname === 'localhost' || 
                       window.location.hostname === '127.0.0.1' || 
                       window.location.hostname === '';
    
    if (isLocalDev) {
        const port = window.location.port;
        
        // If we're on a port other than 8080, assume we're running web files directly
        // (e.g., Live Server on port 5500, http-server on port 8000, etc.)
        if (port && port !== '8080') {
            return 'http://localhost:8080/api';
        }
        
        // If we're on localhost:8080, check if we're going through ingress
        // Ingress will typically serve everything under root, but API under /api
        if (port === '8080' || !port) {
            return '/api';
        }
    }
    
    // For K8s deployment or any other scenario, use relative path
    return '/api';
}

const API_BASE_URL = getApiBaseUrl();
console.log('Using API Base URL:', API_BASE_URL);

// Global state
let currentJobs = [];
let refreshInterval;

// Error code to user-friendly message mapping
const ERROR_MESSAGES = {
    'INVALID_FILE_TYPE': 'Please select a text file (.txt, .md, .csv, .json, .xml, .log)',
    'FILE_TOO_LARGE': 'File is too large. Please select a smaller file.',
    'FILE_MISSING': 'Please select a file to upload.',
    'INVALID_PROCESSING_TYPE': 'Please select a valid processing type.',
    'INVALID_PARAMETERS': 'Invalid parameters for the selected processing type.',
    'INVALID_PARAMETERS_JSON': 'Parameters must be valid JSON format.',
    'FORM_PARSE_ERROR': 'Failed to process the form. Please try again.',
    'FILE_SAVE_ERROR': 'Failed to save the uploaded file. Please try again.',
    'JOB_CREATE_ERROR': 'Failed to create job. Please try again.',
    'QUEUE_ERROR': 'Failed to queue job for processing. Please try again.',
    'JOB_NOT_FOUND': 'Job not found. It may have been deleted.',
    'JOB_NOT_READY': 'Job is not completed yet. Please wait for processing to finish.',
    'RESULT_FILE_MISSING': 'Result file is not available.',
    'RESULT_FILE_NOT_ON_DISK': 'Result file is missing from storage.',
    'RESULT_FILE_READ_ERROR': 'Failed to read result file.',
    'INVALID_JOB_ID': 'Invalid job ID format.',
    'JOB_ID_MISSING': 'Job ID is required.',
    'INVALID_STATUS_FILTER': 'Invalid status filter value.',
    'INVALID_LIMIT': 'Invalid limit value.',
    'INVALID_OFFSET': 'Invalid offset value.',
    'JOB_LIST_ERROR': 'Failed to load jobs list. Please refresh the page.',
    'GENERIC_ERROR': 'An unexpected error occurred. Please try again.'
};

// DOM Elements
const uploadForm = document.getElementById('upload-form');
const processingTypeSelect = document.getElementById('processing-type');
const replaceParams = document.getElementById('replace-params');
const extractParams = document.getElementById('extract-params');
const statusFilter = document.getElementById('status-filter');
const refreshBtn = document.getElementById('refresh-btn');
const jobsGrid = document.getElementById('jobs-grid');
const jobModal = document.getElementById('job-modal');
const modalBody = document.getElementById('modal-body');
const downloadResultBtn = document.getElementById('download-result');
const toast = document.getElementById('toast');
const healthIndicator = document.getElementById('health-indicator');
const healthText = document.getElementById('health-text');

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    checkHealth();
    loadJobs();
    startAutoRefresh();
});

function setupEventListeners() {
    // Form submission
    uploadForm.addEventListener('submit', handleJobSubmission);
    
    // Processing type change
    processingTypeSelect.addEventListener('change', handleProcessingTypeChange);
    
    // Filters and refresh
    statusFilter.addEventListener('change', loadJobs);
    refreshBtn.addEventListener('click', loadJobs);
    
    // Modal controls
    document.querySelectorAll('.modal-close').forEach(btn => {
        btn.addEventListener('click', closeModal);
    });
    
    jobModal.addEventListener('click', (e) => {
        if (e.target === jobModal) closeModal();
    });
    
    downloadResultBtn.addEventListener('click', handleDownloadResult);
    
    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') closeModal();
        if (e.key === 'F5') {
            e.preventDefault();
            loadJobs();
        }
    });
}

function handleProcessingTypeChange(e) {
    const value = e.target.value;
    
    // Hide all parameter sections
    replaceParams.style.display = 'none';
    extractParams.style.display = 'none';
    
    // Show relevant parameter section
    if (value === 'replace') {
        replaceParams.style.display = 'block';
    } else if (value === 'extract') {
        extractParams.style.display = 'block';
    }
}

async function handleJobSubmission(e) {
    e.preventDefault();
    
    const submitBtn = document.getElementById('submit-btn');
    submitBtn.disabled = true;
    submitBtn.textContent = 'Submitting...';
    
    try {
        const formData = new FormData();
        const fileInput = document.getElementById('file-input');
        const processingType = processingTypeSelect.value;
        
        if (!fileInput.files[0]) {
            throw new Error('Please select a file');
        }
        
        formData.append('file', fileInput.files[0]);
        formData.append('processing_type', processingType);
        
        // Add parameters based on processing type
        const parameters = {};
        if (processingType === 'replace') {
            const find = document.getElementById('find-text').value;
            const replaceWith = document.getElementById('replace-text').value;
            if (!find) throw new Error('Find text is required for replace operation');
            parameters.find = find;
            parameters.replace_with = replaceWith;
        } else if (processingType === 'extract') {
            const pattern = document.getElementById('pattern-text').value;
            if (!pattern) throw new Error('Pattern is required for extract operation');
            parameters.pattern = pattern;
        }
        
        if (Object.keys(parameters).length > 0) {
            formData.append('parameters', JSON.stringify(parameters));
        }
        
        const response = await fetch(`${API_BASE_URL}/v1/jobs`, {
            method: 'POST',
            body: formData
        });
        
        if (!response.ok) {
            const error = await response.json();
            const userMessage = ERROR_MESSAGES[error.error_code] || error.error || 'Failed to submit job';
            throw new Error(userMessage);
        }
        
        const job = await response.json();
        showToast(`Job submitted successfully! Job ID: ${job.id}`, 'success');
        
        // Reset form
        uploadForm.reset();
        handleProcessingTypeChange({ target: { value: '' } });
        
        // Refresh jobs list
        loadJobs();
        
    } catch (error) {
        showToast(error.message, 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Submit Job';
    }
}

async function checkHealth() {
    try {
        const response = await fetch(`${API_BASE_URL.replace(/\/api$/, '')}/health`);
        if (response.ok) {
            healthIndicator.className = 'indicator healthy';
            healthText.textContent = 'Service Healthy';
        } else {
            throw new Error('Health check failed');
        }
    } catch (error) {
        console.warn('API health check failed:', error.message);
        console.log('Current API_BASE_URL:', API_BASE_URL);
        console.log('Current location:', window.location.href);
        
        healthIndicator.className = 'indicator unhealthy';
        healthText.textContent = 'Service Unavailable';
        
        // If we're in development and the API is not available, show helpful message
        if (API_BASE_URL.startsWith('http://localhost:8080')) {
            console.log('💡 Tip: Make sure your API is running on localhost:8080 or use port forwarding for K8s');
        }
    }
}

async function loadJobs() {
    try {
        const status = statusFilter.value;
        const params = new URLSearchParams();
        if (status) params.append('status', status);
        params.append('limit', '50');
        
        const response = await fetch(`${API_BASE_URL}/v1/jobs?${params}`);
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            const userMessage = ERROR_MESSAGES[error.error_code] || error.error || 'Failed to load jobs';
            throw new Error(userMessage);
        }
        
        const data = await response.json();
        currentJobs = data.jobs || [];
        renderJobs();
        
    } catch (error) {
        jobsGrid.innerHTML = `<div class="loading">Error loading jobs: ${error.message}</div>`;
    }
}

function renderJobs() {
    if (currentJobs.length === 0) {
        jobsGrid.innerHTML = '<div class="loading">No jobs found</div>';
        return;
    }
    
    jobsGrid.innerHTML = currentJobs.map(job => `
        <div class="job-card" onclick="showJobDetails('${job.id}')">
            <div class="job-header">
                <div class="job-title">${job.original_filename}</div>
                <div class="job-status status-${job.status}">${job.status}</div>
            </div>
            <div class="job-details">
                <div><strong>Type:</strong> ${job.processing_type}</div>
                <div><strong>Created:</strong> ${formatDate(job.created_at)}</div>
                <div><strong>ID:</strong> ${job.id.substring(0, 8)}...</div>
                <div><strong>Worker:</strong> ${job.worker_id || 'Not assigned'}</div>
            </div>
        </div>
    `).join('');
}

async function showJobDetails(jobId) {
    try {
        const response = await fetch(`${API_BASE_URL}/v1/jobs/${jobId}`);
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            const userMessage = ERROR_MESSAGES[error.error_code] || error.error || 'Failed to load job details';
            throw new Error(userMessage);
        }
        
        const job = await response.json();
        renderJobModal(job);
        jobModal.style.display = 'flex';
        
    } catch (error) {
        showToast(`Error loading job details: ${error.message}`, 'error');
    }
}

function renderJobModal(job) {
    const canDownload = job.status === 'succeeded';
    
    modalBody.innerHTML = `
        <div class="detail-grid">
            <div class="detail-row">
                <div class="detail-label">Job ID:</div>
                <div class="detail-value">${job.id}</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Filename:</div>
                <div class="detail-value">${job.original_filename}</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Processing Type:</div>
                <div class="detail-value">${job.processing_type}</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Parameters:</div>
                <div class="detail-value">${formatParameters(job.parameters)}</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Status:</div>
                <div class="detail-value">
                    <span class="job-status status-${job.status}">${job.status}</span>
                </div>
            </div>
            ${job.error_message ? `
            <div class="detail-row">
                <div class="detail-label">Error:</div>
                <div class="detail-value" style="color: var(--error-color);">${job.error_message}</div>
            </div>
            ` : ''}
            <div class="detail-row">
                <div class="detail-label">Created:</div>
                <div class="detail-value">${formatDate(job.created_at)}</div>
            </div>
            ${job.started_at ? `
            <div class="detail-row">
                <div class="detail-label">Started:</div>
                <div class="detail-value">${formatDate(job.started_at)}</div>
            </div>
            ` : ''}
            ${job.completed_at ? `
            <div class="detail-row">
                <div class="detail-label">Completed:</div>
                <div class="detail-value">${formatDate(job.completed_at)}</div>
            </div>
            ` : ''}
            ${job.worker_id ? `
            <div class="detail-row">
                <div class="detail-label">Worker:</div>
                <div class="detail-value">${job.worker_id}</div>
            </div>
            ` : ''}
        </div>
    `;
    
    downloadResultBtn.style.display = canDownload ? 'block' : 'none';
    downloadResultBtn.dataset.jobId = job.id;
}

async function handleDownloadResult() {
    const jobId = downloadResultBtn.dataset.jobId;
    
    try {
        const response = await fetch(`${API_BASE_URL}/v1/jobs/${jobId}/result`);
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            const userMessage = ERROR_MESSAGES[error.error_code] || error.error || 'Failed to download result';
            throw new Error(userMessage);
        }
        
        // Get filename from content-disposition header or create default
        const contentDisposition = response.headers.get('content-disposition');
        let filename = `result_${jobId}.txt`;
        if (contentDisposition) {
            const filenameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/);
            if (filenameMatch) {
                filename = filenameMatch[1].replace(/['"]/g, '');
            }
        }
        
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
        
        showToast('Result downloaded successfully', 'success');
        
    } catch (error) {
        showToast(`Download failed: ${error.message}`, 'error');
    }
}

function closeModal() {
    jobModal.style.display = 'none';
}

function showToast(message, type = 'success') {
    toast.textContent = message;
    toast.className = `toast ${type}`;
    toast.classList.add('show');
    
    // Auto-hide success messages after 4 seconds, but keep error messages longer
    const hideDelay = type === 'error' ? 6000 : 4000;
    setTimeout(() => {
        toast.classList.remove('show');
    }, hideDelay);
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleString();
}

function formatParameters(params) {
    if (!params || Object.keys(params).length === 0) {
        return 'None';
    }
    
    return Object.entries(params)
        .map(([key, value]) => `${key}: ${value}`)
        .join(', ');
}

function startAutoRefresh() {
    // Refresh jobs every 5 seconds
    refreshInterval = setInterval(() => {
        loadJobs();
        checkHealth();
    }, 5000);
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
});