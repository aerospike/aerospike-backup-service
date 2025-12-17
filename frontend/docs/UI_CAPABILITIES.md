# Aerospike Backup Service (ABS) Dashboard - UI Capabilities

## Overview

The Aerospike Backup Service (ABS) Dashboard provides a comprehensive web interface for monitoring and configuring the
ABS. It is structured into distinct Monitoring and Configuration sections, offering a unified platform for operational
oversight and system management.

## Monitoring Dashboard Capabilities

The Monitoring Dashboard enables operators to observe and manage backup and restore activities.

### Key Features:

- **Routine Listing**: Displays a list of all configured backup routines, allowing for focused monitoring of individual
  workflows.
- **Live Activity Feed**: Presents real-time status of currently running backup and restore jobs with the ability to
  cancel active operations.
- **Backup History & Restore**: Provides a historical view of completed backups (Full/Incremental chains) and supports
  initiating restore operations from any valid historical point.
- **Service Logs Viewer**: Offers a filtered view of ABS service logs with live polling, severity filtering, search, and
  export capabilities for troubleshooting.

## Configuration Editor Capabilities

The Configuration Editor provides a visual interface for managing the ABS configuration, designed to reduce errors and
streamline setup.

### Key Features:

- **Section-Based Navigation**: Organizes configuration settings into logical sections: Routines, Clusters, Storage,
  Policies, Secret Agents, and Service settings.
- **Security & Access Control**:
    - **Cluster Security**: Full support for **TLS** configuration (certificates, protocols, cipher suites) and multiple
      authentication modes (Internal, External, PKI).
    - **Encryption Management**: Flexible backup encryption settings allowing keys to be sourced from files, environment
      variables, or **Secret Agents**.
- **Advanced Storage Configuration**:
    - **S3 Capabilities**: Granular control over S3 storage, including **Storage Classes** (Standard, Glacier, etc.),
      performance tuning (part sizes, concurrent connections), and custom endpoints.
    - **Secret Integration**: Seamless integration with Secret Agents for secure credential management across storage
      backends.
- **Policy Management**:
    - **Retention Rules**: Visual configuration for full and incremental backup retention.
    - **Compression Tuning**: Adjustable compression settings, including ZSTD level controls for balancing performance and
      storage.
- **Interactive Connectivity Checks**:
    - **Validation on Demand**: Dedicated "Check Connectivity" buttons are available for **Aerospike Clusters**, **Storage
      Backends**, and **Secret Agents**. This allows operators to immediately verify network reachability and credentials
      before applying the configuration.
- **Smart Scheduling (Cron)**:
    - **Presets**: Offers one-click setup for common schedules (Hourly, Daily, Weekly) alongside a raw editor for advanced
      use cases.
    - **Natural Language Descriptions**: The scheduler translates complex Cron expressions into readable English.
- **Enhanced Validation & Feedback**:
    - **Detailed Error Reporting**: Displays comprehensive error messages from the backend API during configuration
      application, with copy-to-clipboard functionality.
    - **Context-Aware Input**: Dynamic dropdowns and autocompletion for resources, ensuring referenced items (like
      Namespaces and Sets) exist.

This dashboard aims to provide a robust, validated, and user-friendly experience for managing Aerospike backup
operations.
