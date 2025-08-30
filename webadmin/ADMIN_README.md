# Arcadia App Engine Web Admin

A React-based web admin interface for managing the Arcadia App Engine.

## Features

### App Management
- **View Apps**: List all registered apps with their details, tools, and files
- **Submit New Apps**: Upload and compile new Rust apps as WASM modules
- **Run Tools**: Execute tools from registered apps with custom input

### Schedule Management
- **Create Schedules**: Schedule app tools to run one-time or on a recurring basis
- **Manage Schedules**: View, edit, activate/deactivate, and delete schedules
- **View Scheduled Runs**: Monitor execution history and results of scheduled runs

## Prerequisites

- Node.js 14+ and npm
- Arcadia App Engine running on `http://localhost:8080`

## Getting Started

1. **Install dependencies**:
   ```bash
   npm install
   ```

2. **Start the development server**:
   ```bash
   npm start
   ```

3. **Open your browser** to `http://localhost:3000`

## API Configuration

The web admin connects to the Arcadia App Engine API at `http://localhost:8080` by default. 

To change the API base URL, modify the `API_BASE_URL` constant in `src/services/api.ts`.

## Available Scripts

- `npm start` - Runs the app in development mode
- `npm run build` - Builds the app for production
- `npm test` - Launches the test runner
- `npm run eject` - One-way operation to eject from Create React App

## App Engine API Endpoints

The web admin uses these REST API endpoints from the Go app engine:

### App Management
- `GET /list_apps` - List all registered apps
- `POST /run_tool` - Execute a tool from an app
- `POST /submit_app_src` - Submit new app source code for compilation

### Schedule Management
- `GET /list_schedules[?appId=<id>]` - List schedules (optionally filtered by app)
- `GET /get_schedule?id=<id>` - Get specific schedule details
- `POST /schedule_app_run` - Create new schedule
- `PUT /update_schedule?id=<id>` - Update existing schedule
- `DELETE /delete_schedule?id=<id>` - Delete schedule
- `GET /list_scheduled_runs[?schedule_id=<id>]` - List scheduled runs

## Usage

### Submitting a New App

1. Navigate to "Submit App"
2. Fill in:
   - **App ID**: Unique identifier for your app
   - **Version**: Version number (e.g., "1.0.0")
   - **Runtime**: Select "WASM"
   - **Tools**: Define the tools your app provides with their input formats
   - **App Source**: Paste your Rust trait implementation code
3. Click "Submit App" to compile and register

### Running a Tool

1. Navigate to "Run Tool"
2. Select an app and one of its tools
3. Enter JSON input matching the tool's expected format
4. Click "Run Tool" to execute

### Creating a Schedule

1. Navigate to "Schedules" and click "New Schedule"
2. Select the app and tool to schedule
3. Provide JSON input for the tool
4. Choose schedule type:
   - **One-time**: Runs once at the specified time
   - **Recurring**: Runs repeatedly based on recurrence rules
5. Set the scheduled time and recurrence settings if applicable
6. Click "Create Schedule"

## Architecture

### Components
- `AppList` - Displays registered apps and their tools
- `AppSubmit` - Form for submitting new apps
- `ToolRunner` - Interface for executing app tools
- `ScheduleList` - Table view of all schedules with management actions
- `ScheduleForm` - Form for creating/editing schedules
- `ScheduledRunsList` - Historical view of schedule execution results

### Services
- `api.ts` - Axios-based API client with TypeScript interfaces

### Styling
- Custom CSS with responsive design
- Professional admin interface with clear navigation
- Color-coded status indicators and action buttons

## Development

The app uses:
- **React 18** with TypeScript
- **React Router** for client-side routing
- **Axios** for HTTP requests
- **CSS3** for styling (no external UI framework to keep it lightweight)

## Contributing

1. Make changes to components in `src/components/`
2. Update API types in `src/services/api.ts` if needed
3. Test with the running app engine
4. Build and deploy with `npm run build`