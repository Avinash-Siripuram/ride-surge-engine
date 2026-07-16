# Multi-Cloud Deployment Guide

This document provides step-by-step instructions for deploying the Ride-Surge Live Engine on a 100% Free Cloud Stack.

---

## ☁️ Cloud Platforms Setup

The application is deployed across five cloud providers, utilizing their free tiers to run 24/7 without generating costs:

```
┌────────────────────────────────────────────────────────┐
│                   Vercel (Frontend)                    │
└──────────────────────────┬─────────────────────────────┘
                           │ WebSocket / HTTPS
┌──────────────────────────▼─────────────────────────────┐
│                    Render (Backend)                    │
└──────┬───────────────────┬───────────────────┬─────────┘
       │ TCP Pooler        │ SSL (TLS)         │ SASL SSL
┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
│  Supabase   │     │   Upstash   │     │    Aiven    │
│ (PostgreSQL)│     │   (Redis)   │     │   (Kafka)   │
└─────────────┘     └─────────────┘     └─────────────┘
```

---

## 🛠️ Step-by-Step Deploy Guide

### 1. Database (Supabase PostgreSQL)
1. Register a free account at **Supabase**.
2. Create a new project.
3. Retrieve your **Transaction Pooler** connection string:
   * Go to **Settings** ➡️ **Database** ➡️ **Connection Strings** ➡️ **Pooler**.
   * Ensure the mode is set to **Transaction** and the port is **`6543`**.
   * *Note: Port 6543 is IPv4-compatible, which is required because free-tier application containers often block direct IPv6 database hosts (port 5432).*

### 2. Cache & Geo-Index (Upstash Redis)
1. Register at **Upstash** (Serverless Database).
2. Create a free **Redis** database.
3. Copy the **Redis Connect URL** starting with `rediss://` (for secure TLS).

### 3. Event Streams (Aiven Apache Kafka)
1. Register at **Aiven**.
2. Create a free **Apache Kafka** service.
3. Generate topics (or allow the backend Go admin client to automatically create them on boot).
4. Retrieve your service credentials:
   * **Bootstrap Broker URL** (Format: `[service-name]-[project].k.aivencloud.com:[port]`)
   * **SASL Username** (`avnadmin`)
   * **SASL Password**

### 4. Backend (Render Web Service)
1. Go to **Render**.
2. Click **New +** ➡️ **Web Service** ➡️ Connect your GitHub repository.
3. Configure the following build details:
   * **Name**: `ride-surge-backend`
   * **Language**: `Go`
   * **Root Directory**: `backend`
   * **Instance Type**: `Free`
4. Expand the **Advanced** section and add the 5 Environment Variables:
   * `DATABASE_URL` (Supabase transaction pooler URL)
   * `REDIS_URL` (Upstash connection string)
   * `KAFKA_BROKER` (Aiven bootstrap host)
   * `KAFKA_SASL_USER` (`avnadmin`)
   * `KAFKA_SASL_PASS` (Aiven credentials)
5. Click **Deploy Web Service** (no credit card required!).

### 5. Frontend (Vercel)
1. Go to **Vercel**.
2. Click **Add New** ➡️ **Project** ➡️ Import your GitHub repository.
3. Vercel automatically detects the Angular framework. Leave the settings at their defaults.
4. Click **Deploy**. Vercel will compile and host your static site.

---

## 📋 Environment Variables Reference Checklist

Ensure the environment variables on Render are exactly as follows:

| Variable Name | Description | Example Pattern |
| :--- | :--- | :--- |
| **`DATABASE_URL`** | Supabase Postgres Transaction Pooler string | `postgresql://postgres.[PROJECT_ID]:[YOUR_PASSWORD]@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres` |
| **`REDIS_URL`** | Upstash Redis TLS connection string | `rediss://default:[YOUR_PASSWORD]@charming-python-97852.upstash.io:6379` |
| **`KAFKA_BROKER`** | Aiven Kafka broker address with `.k` subdomain | `kafka-[your-service-id].k.aivencloud.com:[port]` |
| **`KAFKA_SASL_USER`** | SASL username for Kafka SCRAM authentication | `avnadmin` |
| **`KAFKA_SASL_PASS`** | SASL password for Kafka SCRAM authentication | `[YOUR_AIVEN_PASSWORD]` |

---

## 🔍 Network Compatibility Gotchas

*   **IPv6 Limitations**: Free hosting containers (like Render or Back4app) usually route outgoing traffic through NAT proxies that do not support IPv6. Because Supabase's default direct database domain is IPv6-only, using the pooler domain (`*.pooler.supabase.com` on port `6543`) resolves to IPv4 and avoids connection timeouts.
*   **Aiven Broker subdomains**: Verify that your `KAFKA_BROKER` includes the `.k.` subdomain block (e.g. `[service-name].[project].k.aivencloud.com`). Standard `.aivencloud.com` brokers will trigger DNS lookup failures in container environments.
