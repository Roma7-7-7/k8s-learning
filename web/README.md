# Web UI for File Processing Service

A minimalistic static web interface for the file processing API service. Built with vanilla HTML, CSS, and JavaScript - no build process required.

## Features

- **File Upload**: Submit files for processing with various processing types
- **Job Management**: View, filter, and monitor job status in real-time  
- **Real-time Updates**: Auto-refresh every 5 seconds
- **Result Download**: Download processed files directly from the UI
- **Responsive Design**: Works on desktop and mobile devices
- **Health Monitoring**: Shows API service health status

## Processing Types Supported

- **Word Count**: Count words in the file
- **Line Count**: Count lines in the file  
- **Uppercase**: Convert text to uppercase
- **Lowercase**: Convert text to lowercase
- **Find & Replace**: Replace text with custom find/replace parameters
- **Extract Pattern**: Extract text using regex patterns

## Running the Web UI

Since this is a static web application, you can run it in multiple ways:

### Option 1: Simple HTTP Server (Python)
```bash
cd web
python3 -m http.server 3000
```
Then open http://localhost:3000

### Option 2: Simple HTTP Server (Node.js)
```bash
cd web
npx http-server -p 3000
```

### Option 3: Any Web Server
Serve the `web/` directory with any web server (nginx, Apache, etc.)

## Configuration

Update the `API_BASE_URL` in `app.js` to match your API server:

```javascript
const API_BASE_URL = 'http://localhost:8080'; // Change to your API server
```

## Architecture

- **index.html**: Semantic HTML5 structure with accessibility features
- **styles.css**: Modern CSS with CSS Grid, Flexbox, and CSS custom properties
- **app.js**: Vanilla JavaScript with fetch API for HTTP requests

## Browser Compatibility

- Modern browsers with ES6+ support
- Uses fetch API, CSS Grid, and CSS custom properties
- Mobile responsive design

## Security Notes

- CORS must be enabled on the API server
- File uploads are handled by the API server
- No sensitive data stored in browser localStorage