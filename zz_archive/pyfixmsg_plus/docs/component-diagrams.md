# PyFixMsg Plus Component Architecture Diagrams
## Visual System Design and Component Relationships

**Document Version:** 1.0  
**Date:** July 26, 2025  
**Purpose:** Comprehensive architectural diagrams for PyFixMsg Plus system components  

---

## Overview

This document provides detailed architectural diagrams showing the relationships, data flow, and deployment patterns for PyFixMsg Plus components across all development phases.

## Table of Contents

1. [High-Level System Architecture](#high-level-system-architecture)
2. [Core Component Relationships](#core-component-relationships)
3. [Message Flow Diagrams](#message-flow-diagrams)
4. [Data Storage Architecture](#data-storage-architecture)
5. [Session State Management](#session-state-management)
6. [Network Layer Architecture](#network-layer-architecture)
7. [Deployment Architectures](#deployment-architectures)
8. [Phase Evolution Diagrams](#phase-evolution-diagrams)

---

## High-Level System Architecture

### Core System Overview

```mermaid
graph TB
    subgraph "External Systems"
        COUNTERPARTY[FIX Counterparty]
        TRADING_PLATFORM[Trading Platform]
        MARKET_DATA[Market Data Provider]
        ADMIN_CONSOLE[Admin Console]
    end
    
    subgraph "PyFixMsg Plus Engine"
        subgraph "Application Layer"
            USER_APP[User Application]
            TRADING_APP[Trading Application]
            CLI_TOOLS[CLI Tools]
        end
        
        subgraph "Core Engine"
            FIX_ENGINE[FixEngine]
            CONFIG_MGR[ConfigManager]
            STATE_MACHINE[StateMachine]
            MSG_PROCESSOR[MessageProcessor]
        end
        
        subgraph "Protocol Layer"
            MSG_HANDLERS[Message Handlers]
            NETWORK_LAYER[Network Layer]
            HEARTBEAT_MGR[Heartbeat Manager]
            SEQUENCE_MGR[Sequence Manager]
        end
        
        subgraph "Storage Layer"
            MSG_STORE_FACTORY[MessageStoreFactory]
            SQLITE_STORE[SQLite Store]
            AIOSQLITE_STORE[AioSQLite Store]
            ENTERPRISE_STORES[Enterprise Stores]
        end
        
        subgraph "Infrastructure"
            CRYPTO[Encryption/Decryption]
            SCHEDULER[Task Scheduler]
            MONITORING[Monitoring]
            LOGGING[Logging]
        end
    end
    
    COUNTERPARTY <--> NETWORK_LAYER
    TRADING_PLATFORM <--> USER_APP
    MARKET_DATA --> TRADING_APP
    ADMIN_CONSOLE <--> CLI_TOOLS
    
    USER_APP --> FIX_ENGINE
    TRADING_APP --> FIX_ENGINE
    CLI_TOOLS --> FIX_ENGINE
    
    FIX_ENGINE --> CONFIG_MGR
    FIX_ENGINE --> STATE_MACHINE
    FIX_ENGINE --> MSG_PROCESSOR
    FIX_ENGINE --> NETWORK_LAYER
    
    MSG_PROCESSOR --> MSG_HANDLERS
    MSG_HANDLERS --> HEARTBEAT_MGR
    MSG_HANDLERS --> SEQUENCE_MGR
    
    FIX_ENGINE --> MSG_STORE_FACTORY
    MSG_STORE_FACTORY --> SQLITE_STORE
    MSG_STORE_FACTORY --> AIOSQLITE_STORE
    MSG_STORE_FACTORY --> ENTERPRISE_STORES
    
    CONFIG_MGR --> CRYPTO
    FIX_ENGINE --> SCHEDULER
    FIX_ENGINE --> MONITORING
    FIX_ENGINE --> LOGGING
    
    classDef external fill:#FFE4E1,stroke:#DC143C,stroke-width:2px
    classDef application fill:#E0FFFF,stroke:#008B8B,stroke-width:2px
    classDef core fill:#90EE90,stroke:#006400,stroke-width:2px
    classDef protocol fill:#FFE4B5,stroke:#FF8C00,stroke-width:2px
    classDef storage fill:#DDA0DD,stroke:#9932CC,stroke-width:2px
    classDef infrastructure fill:#F0F8FF,stroke:#4682B4,stroke-width:2px
    
    class COUNTERPARTY,TRADING_PLATFORM,MARKET_DATA,ADMIN_CONSOLE external
    class USER_APP,TRADING_APP,CLI_TOOLS application
    class FIX_ENGINE,CONFIG_MGR,STATE_MACHINE,MSG_PROCESSOR core
    class MSG_HANDLERS,NETWORK_LAYER,HEARTBEAT_MGR,SEQUENCE_MGR protocol
    class MSG_STORE_FACTORY,SQLITE_STORE,AIOSQLITE_STORE,ENTERPRISE_STORES storage
    class CRYPTO,SCHEDULER,MONITORING,LOGGING infrastructure
```

## Core Component Relationships

### FixEngine Internal Structure

```mermaid
graph LR
    subgraph "FixEngine Core"
        ENGINE[FixEngine]
        
        subgraph "Session Management"
            STATE[StateMachine]
            LIFECYCLE[Session Lifecycle]
            HEARTBEAT[Heartbeat System]
        end
        
        subgraph "Message Processing"
            PROCESSOR[MessageProcessor]
            ROUTER[Message Router]
            HANDLERS[Handler Registry]
        end
        
        subgraph "Network Management"
            NETWORK[Network Layer]
            ACCEPTOR[Acceptor Mode]
            INITIATOR[Initiator Mode]
        end
        
        subgraph "Storage Management"
            STORE_MGR[Store Manager]
            SEQ_MGR[Sequence Manager]
            PERSIST[Persistence Layer]
        end
        
        subgraph "Configuration"
            CONFIG[ConfigManager]
            SECURITY[Security/Crypto]
            VALIDATION[Config Validation]
        end
    end
    
    ENGINE --> STATE
    ENGINE --> PROCESSOR
    ENGINE --> NETWORK
    ENGINE --> STORE_MGR
    ENGINE --> CONFIG
    
    STATE --> LIFECYCLE
    STATE --> HEARTBEAT
    
    PROCESSOR --> ROUTER
    ROUTER --> HANDLERS
    
    NETWORK --> ACCEPTOR
    NETWORK --> INITIATOR
    
    STORE_MGR --> SEQ_MGR
    STORE_MGR --> PERSIST
    
    CONFIG --> SECURITY
    CONFIG --> VALIDATION
    
    HEARTBEAT --> NETWORK
    HANDLERS --> STORE_MGR
    SEQ_MGR --> STATE
    SECURITY --> CONFIG
```

### Message Handler Architecture

```mermaid
graph TB
    subgraph "Message Processor"
        MSG_PROC[MessageProcessor]
        ROUTER[Message Router]
        DISPATCHER[Handler Dispatcher]
    end
    
    subgraph "Session-Level Handlers"
        LOGON_H[LogonHandler]
        LOGOUT_H[LogoutHandler]
        HEARTBEAT_H[HeartbeatHandler]
        TESTREQ_H[TestRequestHandler]
        RESEND_H[ResendRequestHandler]
        SEQRESET_H[SequenceResetHandler]
        REJECT_H[RejectHandler]
    end
    
    subgraph "Application-Level Handlers"
        ORDER_H[NewOrderHandler]
        CANCEL_H[CancelOrderHandler]
        REPLACE_H[OrderReplaceHandler]
        EXEC_H[ExecutionReportHandler]
        MULTILEG_H[MultilegHandlers]
        CUSTOM_H[Custom Handlers]
    end
    
    subgraph "Handler Support"
        VALIDATOR[Message Validator]
        TRANSFORMER[Message Transformer]
        ERROR_H[Error Handler]
        METRICS[Metrics Collector]
    end
    
    MSG_PROC --> ROUTER
    ROUTER --> DISPATCHER
    
    DISPATCHER --> LOGON_H
    DISPATCHER --> LOGOUT_H
    DISPATCHER --> HEARTBEAT_H
    DISPATCHER --> TESTREQ_H
    DISPATCHER --> RESEND_H
    DISPATCHER --> SEQRESET_H
    DISPATCHER --> REJECT_H
    
    DISPATCHER --> ORDER_H
    DISPATCHER --> CANCEL_H
    DISPATCHER --> REPLACE_H
    DISPATCHER --> EXEC_H
    DISPATCHER --> MULTILEG_H
    DISPATCHER --> CUSTOM_H
    
    LOGON_H --> VALIDATOR
    ORDER_H --> VALIDATOR
    CANCEL_H --> TRANSFORMER
    EXEC_H --> ERROR_H
    
    VALIDATOR --> METRICS
    TRANSFORMER --> METRICS
    ERROR_H --> METRICS
    
    classDef processor fill:#87CEEB,stroke:#4682B4,stroke-width:2px
    classDef session fill:#98FB98,stroke:#228B22,stroke-width:2px
    classDef application fill:#FFB6C1,stroke:#DC143C,stroke-width:2px
    classDef support fill:#DDA0DD,stroke:#9932CC,stroke-width:2px
    
    class MSG_PROC,ROUTER,DISPATCHER processor
    class LOGON_H,LOGOUT_H,HEARTBEAT_H,TESTREQ_H,RESEND_H,SEQRESET_H,REJECT_H session
    class ORDER_H,CANCEL_H,REPLACE_H,EXEC_H,MULTILEG_H,CUSTOM_H application
    class VALIDATOR,TRANSFORMER,ERROR_H,METRICS support
```

## Message Flow Diagrams

### Complete Message Processing Flow

```mermaid
sequenceDiagram
    participant C as Counterparty
    participant N as NetworkLayer
    participant E as FixEngine
    participant P as MessageProcessor
    participant H as MessageHandler
    participant S as MessageStore
    participant A as Application
    participant SM as StateMachine
    
    Note over C,SM: Incoming Message Flow
    
    C->>N: Raw FIX Message
    N->>E: Parsed Message Bytes
    E->>E: Message Framing
    E->>E: Basic Validation
    
    alt Valid Message Structure
        E->>S: Store Incoming Message
        E->>P: Route Message for Processing
        P->>H: Dispatch to Handler
        
        alt Session-Level Message
            H->>SM: Update Session State
            H->>E: Generate Response (if needed)
        else Application-Level Message
            H->>A: Forward to Application
            A->>H: Application Response
            H->>E: Generate Response Message
        end
        
        alt Response Required
            E->>S: Store Outgoing Message
            E->>N: Send Response
            N->>C: Raw FIX Response
        end
    else Invalid Message
        E->>H: Route to RejectHandler
        H->>E: Generate Reject Message
        E->>S: Store Reject Message
        E->>N: Send Reject
        N->>C: Reject Response
    end
    
    Note over C,SM: Error Handling
    
    alt Network Error
        N->>E: Connection Lost
        E->>SM: Trigger Disconnect
        SM->>E: Cleanup Resources
    end
```

### Session Establishment Flow

```mermaid
sequenceDiagram
    participant I as Initiator
    participant A as Acceptor
    participant E1 as Engine (Initiator)
    participant E2 as Engine (Acceptor)
    participant S1 as Store (Initiator)
    participant S2 as Store (Acceptor)
    
    Note over I,S2: Session Establishment
    
    I->>E1: start()
    E1->>E1: Connect to Acceptor
    E1->>A: TCP Connection
    A->>E2: Accept Connection
    E2->>E2: Set State: AWAITING_LOGON
    
    E1->>E1: Create Logon Message
    E1->>S1: Store Outgoing Logon
    E1->>A: Send Logon (35=A)
    
    A->>E2: Receive Logon
    E2->>S2: Store Incoming Logon
    E2->>E2: Validate Logon
    E2->>E2: Set State: ACTIVE
    E2->>E2: Create Logon Response
    E2->>S2: Store Outgoing Response
    E2->>I: Send Logon Response
    
    I->>E1: Receive Logon Response
    E1->>S1: Store Incoming Response
    E1->>E1: Set State: ACTIVE
    
    Note over I,S2: Heartbeat Initialization
    
    E1->>E1: Start Heartbeat Timer
    E2->>E2: Start Heartbeat Timer
    
    Note over I,S2: Session Active - Ready for Messages
```

## Data Storage Architecture

### Message Store Interface Hierarchy

```mermaid
classDiagram
    class MessageStoreInterface {
        <<interface>>
        +store_message(version, sender, target, seqnum, message)
        +get_message(version, sender, target, seqnum)
        +get_next_incoming_sequence_number()
        +get_next_outgoing_sequence_number()
        +increment_incoming_sequence_number()
        +increment_outgoing_sequence_number()
        +reset_sequence_numbers()
        +close()
    }
    
    class DatabaseMessageStore {
        -db_path: str
        -conn: sqlite3.Connection
        -lock: asyncio.Lock
        +initialize()
        +create_table()
        +load_sequence_numbers()
        +store_message()
        +get_message()
        +get_messages_range()
    }
    
    class DatabaseMessageStoreAioSqlite {
        -db_path: str
        -conn: aiosqlite.Connection
        -lock: asyncio.Lock
        +initialize()
        +create_table()
        +load_sequence_numbers()
        +store_message()
        +get_message()
        +get_messages_range()
    }
    
    class PostgreSQLMessageStore {
        -connection_pool: asyncpg.Pool
        -config: dict
        +initialize()
        +create_tables()
        +partition_management()
        +bulk_insert()
    }
    
    class RedisMessageStore {
        -redis_client: aioredis.Redis
        -ttl_config: dict
        +initialize()
        +set_expiration()
        +pub_sub_notify()
        +cluster_support()
    }
    
    class MessageStoreFactory {
        +get_message_store(type, config)
        -validate_config(config)
        -create_store_instance(type, config)
    }
    
    MessageStoreInterface <|-- DatabaseMessageStore
    MessageStoreInterface <|-- DatabaseMessageStoreAioSqlite
    MessageStoreInterface <|-- PostgreSQLMessageStore
    MessageStoreInterface <|-- RedisMessageStore
    
    MessageStoreFactory --> MessageStoreInterface : creates
    
    note for DatabaseMessageStore "Phase 1: Synchronous SQLite\nDevelopment & Testing"
    note for DatabaseMessageStoreAioSqlite "Phase 1: Asynchronous SQLite\nProduction Ready"
    note for PostgreSQLMessageStore "Phase 3: Enterprise Backend\nHigh Performance & Clustering"
    note for RedisMessageStore "Phase 3: High-Speed Backend\nHFT & Caching"
```

### Data Flow and Persistence Patterns

```mermaid
graph TB
    subgraph "Message Processing"
        INCOMING[Incoming Message]
        OUTGOING[Outgoing Message]
        VALIDATION[Message Validation]
        ROUTING[Message Routing]
    end
    
    subgraph "Storage Decision Layer"
        FACTORY[MessageStoreFactory]
        CONFIG[Storage Configuration]
        TYPE_SELECTOR[Backend Type Selector]
    end
    
    subgraph "Storage Backends"
        subgraph "Development (Phase 1)"
            SQLITE[SQLite Store]
            AIOSQLITE[AioSQLite Store]
        end
        
        subgraph "Enterprise (Phase 3)"
            POSTGRES[PostgreSQL Store]
            MYSQL[MySQL Store]
            REDIS[Redis Store]
            MONGODB[MongoDB Store]
        end
        
        subgraph "Testing"
            MEMORY[In-Memory Store]
            MOCK[Mock Store]
        end
    end
    
    subgraph "Storage Features"
        PARTITIONING[Data Partitioning]
        REPLICATION[Data Replication]
        BACKUP[Backup & Recovery]
        ARCHIVAL[Data Archival]
    end
    
    INCOMING --> VALIDATION
    OUTGOING --> VALIDATION
    VALIDATION --> ROUTING
    ROUTING --> FACTORY
    
    FACTORY --> CONFIG
    CONFIG --> TYPE_SELECTOR
    
    TYPE_SELECTOR --> SQLITE
    TYPE_SELECTOR --> AIOSQLITE
    TYPE_SELECTOR --> POSTGRES
    TYPE_SELECTOR --> MYSQL
    TYPE_SELECTOR --> REDIS
    TYPE_SELECTOR --> MONGODB
    TYPE_SELECTOR --> MEMORY
    TYPE_SELECTOR --> MOCK
    
    POSTGRES --> PARTITIONING
    MYSQL --> REPLICATION
    REDIS --> BACKUP
    MONGODB --> ARCHIVAL
    
    classDef processing fill:#87CEEB,stroke:#4682B4,stroke-width:2px
    classDef decision fill:#FFE4B5,stroke:#FF8C00,stroke-width:2px
    classDef phase1 fill:#90EE90,stroke:#006400,stroke-width:2px
    classDef phase3 fill:#DDA0DD,stroke:#9932CC,stroke-width:2px
    classDef testing fill:#F0F8FF,stroke:#708090,stroke-width:2px
    classDef features fill:#FFB6C1,stroke:#DC143C,stroke-width:2px
    
    class INCOMING,OUTGOING,VALIDATION,ROUTING processing
    class FACTORY,CONFIG,TYPE_SELECTOR decision
    class SQLITE,AIOSQLITE phase1
    class POSTGRES,MYSQL,REDIS,MONGODB phase3
    class MEMORY,MOCK testing
    class PARTITIONING,REPLICATION,BACKUP,ARCHIVAL features
```

## Session State Management

### State Machine Detailed View

```mermaid
stateDiagram-v2
    [*] --> DISCONNECTED
    
    DISCONNECTED --> CONNECTING : start() / initialize_connection()
    CONNECTING --> LOGON_IN_PROGRESS : connection_established / send_logon()
    CONNECTING --> DISCONNECTED : connection_failed / cleanup_resources()
    
    LOGON_IN_PROGRESS --> ACTIVE : logon_accepted / start_heartbeat()
    LOGON_IN_PROGRESS --> DISCONNECTED : logon_rejected / disconnect()
    LOGON_IN_PROGRESS --> DISCONNECTED : timeout / force_disconnect()
    
    ACTIVE --> LOGOUT_IN_PROGRESS : logout_initiated / send_logout()
    ACTIVE --> DISCONNECTED : network_error / emergency_disconnect()
    ACTIVE --> DISCONNECTED : fatal_error / force_disconnect()
    
    LOGOUT_IN_PROGRESS --> DISCONNECTED : logout_confirmed / graceful_shutdown()
    LOGOUT_IN_PROGRESS --> DISCONNECTED : timeout / force_disconnect()
    
    DISCONNECTED --> RECONNECTING : retry_enabled / schedule_retry()
    RECONNECTING --> CONNECTING : retry_attempt / attempt_connection()
    RECONNECTING --> DISCONNECTED : max_retries_reached / abandon_connection()
    
    state ACTIVE {
        [*] --> HEARTBEAT_ACTIVE
        HEARTBEAT_ACTIVE --> AWAITING_RESPONSE : send_test_request()
        AWAITING_RESPONSE --> HEARTBEAT_ACTIVE : response_received()
        AWAITING_RESPONSE --> HEARTBEAT_FAILED : timeout()
        HEARTBEAT_FAILED --> [*] : trigger_disconnect()
        
        --
        
        [*] --> MESSAGE_PROCESSING
        MESSAGE_PROCESSING --> SEQUENCE_GAP : gap_detected()
        SEQUENCE_GAP --> MESSAGE_PROCESSING : gap_filled()
        SEQUENCE_GAP --> RESEND_REQUESTED : send_resend_request()
        RESEND_REQUESTED --> MESSAGE_PROCESSING : messages_received()
    }
    
    note right of ACTIVE
        During ACTIVE state:
        - Process application messages
        - Monitor heartbeats
        - Handle sequence gaps
        - Manage resend requests
    end note
    
    note right of DISCONNECTED
        During DISCONNECTED:
        - Close network connections
        - Save session state
        - Clean up resources
        - Log session statistics
    end note
```

### State Transition Events and Actions

```mermaid
graph LR
    subgraph "External Events"
        USER_START["User start()"]
        NETWORK_CONNECT["Network Connected"]
        NETWORK_ERROR["Network Error"]
        MSG_RECEIVED["Message Received"]
        TIMEOUT_EVENT["Timer Timeout"]
        USER_STOP["User stop()"]
    end
    
    subgraph "State Machine"
        STATE_MGR["StateMachine"]
        CURRENT_STATE["Current State"]
        EVENT_PROCESSOR["Event Processor"]
        TRANSITION_VALIDATOR["Transition Validator"]
    end
    
    subgraph "Actions"
        NETWORK_ACTIONS["Network Actions"]
        MESSAGE_ACTIONS["Message Actions"]
        CLEANUP_ACTIONS["Cleanup Actions"]
        LOGGING_ACTIONS["Logging Actions"]
        CALLBACK_ACTIONS["Callback Actions"]
    end
    
    subgraph "Side Effects"
        NOTIFY_APP["Notify Application"]
        UPDATE_METRICS["Update Metrics"]
        PERSIST_STATE["Persist State"]
        SCHEDULE_TASKS["Schedule Tasks"]
    end
    
    USER_START --> EVENT_PROCESSOR
    NETWORK_CONNECT --> EVENT_PROCESSOR
    NETWORK_ERROR --> EVENT_PROCESSOR
    MSG_RECEIVED --> EVENT_PROCESSOR
    TIMEOUT_EVENT --> EVENT_PROCESSOR
    USER_STOP --> EVENT_PROCESSOR
    
    EVENT_PROCESSOR --> TRANSITION_VALIDATOR
    TRANSITION_VALIDATOR --> CURRENT_STATE
    CURRENT_STATE --> STATE_MGR
    
    STATE_MGR --> NETWORK_ACTIONS
    STATE_MGR --> MESSAGE_ACTIONS
    STATE_MGR --> CLEANUP_ACTIONS
    STATE_MGR --> LOGGING_ACTIONS
    STATE_MGR --> CALLBACK_ACTIONS
    
    NETWORK_ACTIONS --> NOTIFY_APP
    MESSAGE_ACTIONS --> UPDATE_METRICS
    CLEANUP_ACTIONS --> PERSIST_STATE
    LOGGING_ACTIONS --> SCHEDULE_TASKS
    CALLBACK_ACTIONS --> NOTIFY_APP
    
    classDef events fill:#FFE4B5,stroke:#FF8C00,stroke-width:2px
    classDef statemachine fill:#87CEEB,stroke:#4682B4,stroke-width:2px
    classDef actions fill:#98FB98,stroke:#228B22,stroke-width:2px
    classDef sideeffects fill:#DDA0DD,stroke:#9932CC,stroke-width:2px
    
    class USER_START,NETWORK_CONNECT,NETWORK_ERROR,MSG_RECEIVED,TIMEOUT_EVENT,USER_STOP events
    class STATE_MGR,CURRENT_STATE,EVENT_PROCESSOR,TRANSITION_VALIDATOR statemachine
    class NETWORK_ACTIONS,MESSAGE_ACTIONS,CLEANUP_ACTIONS,LOGGING_ACTIONS,CALLBACK_ACTIONS actions
    class NOTIFY_APP,UPDATE_METRICS,PERSIST_STATE,SCHEDULE_TASKS sideeffects
```

## Network Layer Architecture

### Network Component Structure

```mermaid
graph TB
    subgraph "Network Layer"
        NETWORK_MGR[Network Manager]
        
        subgraph "Connection Management"
            CONN_POOL[Connection Pool]
            CONN_FACTORY[Connection Factory]
            CONN_MONITOR[Connection Monitor]
        end
        
        subgraph "Transport Layer"
            TCP_TRANSPORT[TCP Transport]
            SSL_TRANSPORT[SSL/TLS Transport]
            WEBSOCKET_TRANSPORT[WebSocket Transport]
        end
        
        subgraph "Protocol Handling"
            MSG_FRAMER[Message Framer]
            MSG_PARSER[Message Parser]
            CODEC[Message Codec]
        end
        
        subgraph "Mode Implementations"
            ACCEPTOR[Acceptor Mode]
            INITIATOR[Initiator Mode]
            BOTH_MODE[Dual Mode]
        end
        
        subgraph "Quality of Service"
            RECONNECT_MGR[Reconnection Manager]
            HEARTBEAT_MONITOR[Heartbeat Monitor]
            LATENCY_MONITOR[Latency Monitor]
            BANDWIDTH_MGR[Bandwidth Manager]
        end
    end
    
    NETWORK_MGR --> CONN_POOL
    NETWORK_MGR --> TCP_TRANSPORT
    NETWORK_MGR --> MSG_FRAMER
    NETWORK_MGR --> ACCEPTOR
    NETWORK_MGR --> RECONNECT_MGR
    
    CONN_POOL --> CONN_FACTORY
    CONN_POOL --> CONN_MONITOR
    
    TCP_TRANSPORT --> SSL_TRANSPORT
    TCP_TRANSPORT --> WEBSOCKET_TRANSPORT
    
    MSG_FRAMER --> MSG_PARSER
    MSG_PARSER --> CODEC
    
    ACCEPTOR --> INITIATOR
    INITIATOR --> BOTH_MODE
    
    RECONNECT_MGR --> HEARTBEAT_MONITOR
    HEARTBEAT_MONITOR --> LATENCY_MONITOR
    LATENCY_MONITOR --> BANDWIDTH_MGR
    
    classDef management fill:#87CEEB,stroke:#4682B4,stroke-width:2px
    classDef transport fill:#98FB98,stroke:#228B22,stroke-width:2px
    classDef protocol fill:#FFB6C1,stroke:#DC143C,stroke-width:2px
    classDef modes fill:#DDA0DD,stroke:#9932CC,stroke-width:2px
    classDef qos fill:#F0E68C,stroke:#DAA520,stroke-width:2px
    
    class NETWORK_MGR,CONN_POOL,CONN_FACTORY,CONN_MONITOR management
    class TCP_TRANSPORT,SSL_TRANSPORT,WEBSOCKET_TRANSPORT transport
    class MSG_FRAMER,MSG_PARSER,CODEC protocol
    class ACCEPTOR,INITIATOR,BOTH_MODE modes
    class RECONNECT_MGR,HEARTBEAT_MONITOR,LATENCY_MONITOR,BANDWIDTH_MGR qos
```

### Connection Lifecycle Management

```mermaid
sequenceDiagram
    participant App as Application
    participant Net as NetworkManager
    participant Pool as ConnectionPool
    participant Mon as ConnectionMonitor
    participant Recon as ReconnectionManager
    
    Note over App,Recon: Connection Establishment
    
    App->>Net: start_network()
    Net->>Pool: initialize_pool()
    Pool->>Pool: create_initial_connections()
    Pool->>Mon: register_connections()
    
    loop Connection Monitoring
        Mon->>Mon: check_connection_health()
        
        alt Connection Healthy
            Mon->>Mon: update_metrics()
        else Connection Failed
            Mon->>Pool: mark_connection_failed()
            Pool->>Recon: schedule_reconnection()
            
            Recon->>Recon: exponential_backoff()
            Recon->>Pool: attempt_reconnection()
            
            alt Reconnection Success
                Pool->>Mon: register_new_connection()
                Mon->>App: notify_connection_restored()
            else Reconnection Failed
                Recon->>Recon: increment_retry_count()
                
                alt Max Retries Reached
                    Recon->>App: notify_connection_failed()
                else Continue Retrying
                    Recon->>Recon: schedule_next_retry()
                end
            end
        end
    end
    
    Note over App,Recon: Graceful Shutdown
    
    App->>Net: stop_network()
    Net->>Pool: drain_connections()
    Pool->>Mon: stop_monitoring()
    Mon->>Recon: cancel_reconnections()
```

## Deployment Architectures

### Single Instance Deployment (Phase 1-2)

```mermaid
graph TB
    subgraph "Single Host Deployment"
        subgraph "Application Process"
            APP[Python Application]
            ENGINE[PyFixMsg Plus Engine]
            CONFIG[Configuration Files]
        end
        
        subgraph "Local Storage"
            SQLITE[(SQLite Database)]
            LOGS[Log Files]
            STATE[State Files]
        end
        
        subgraph "Monitoring"
            LOCAL_METRICS[Local Metrics]
            FILE_LOGS[File Logging]
        end
    end
    
    subgraph "External Connections"
        COUNTERPARTY[FIX Counterparty]
        ADMIN[Admin Interface]
    end
    
    APP --> ENGINE
    ENGINE --> CONFIG
    ENGINE --> SQLITE
    ENGINE --> LOGS
    ENGINE --> STATE
    ENGINE --> LOCAL_METRICS
    ENGINE --> FILE_LOGS
    
    COUNTERPARTY <--> ENGINE
    ADMIN <--> APP
    
    classDef application fill:#87CEEB,stroke:#4682B4,stroke-width:2px
    classDef storage fill:#98FB98,stroke:#228B22,stroke-width:2px
    classDef monitoring fill:#FFE4B5,stroke:#FF8C00,stroke-width:2px
    classDef external fill:#FFB6C1,stroke:#DC143C,stroke-width:2px
    
    class APP,ENGINE,CONFIG application
    class SQLITE,LOGS,STATE storage
    class LOCAL_METRICS,FILE_LOGS monitoring
    class COUNTERPARTY,ADMIN external
```

### High-Availability Deployment (Phase 3)

```mermaid
graph TB
    subgraph "Load Balancer Tier"
        LB[HAProxy/NGINX]
        HEALTH_CHECK[Health Checks]
    end
    
    subgraph "Application Tier"
        subgraph "Primary Cluster"
            APP1[PyFixMsg Plus Node 1]
            APP2[PyFixMsg Plus Node 2]
            APP3[PyFixMsg Plus Node 3]
        end
        
        subgraph "Standby Cluster"
            STANDBY1[Standby Node 1]
            STANDBY2[Standby Node 2]
        end
    end
    
    subgraph "Data Tier"
        subgraph "Primary Storage"
            PG_PRIMARY[(PostgreSQL Primary)]
            PG_REPLICA1[(PostgreSQL Replica 1)]
            PG_REPLICA2[(PostgreSQL Replica 2)]
        end
        
        subgraph "Cache Layer"
            REDIS_CLUSTER[(Redis Cluster)]
        end
        
        subgraph "Message Queue"
            KAFKA_CLUSTER[Kafka Cluster]
        end
    end
    
    subgraph "Monitoring Tier"
        PROMETHEUS[(Prometheus)]
        GRAFANA[Grafana]
        ALERTMANAGER[AlertManager]
        ELK_STACK[ELK Stack]
    end
    
    subgraph "External Systems"
        COUNTERPARTIES[FIX Counterparties]
        TRADING_SYSTEMS[Trading Systems]
        ADMIN_CONSOLE[Admin Console]
    end
    
    LB --> HEALTH_CHECK
    HEALTH_CHECK --> APP1
    HEALTH_CHECK --> APP2
    HEALTH_CHECK --> APP3
    
    LB --> APP1
    LB --> APP2
    LB --> APP3
    
    APP1 --> PG_PRIMARY
    APP2 --> PG_PRIMARY
    APP3 --> PG_PRIMARY
    
    PG_PRIMARY --> PG_REPLICA1
    PG_PRIMARY --> PG_REPLICA2
    
    APP1 --> REDIS_CLUSTER
    APP2 --> REDIS_CLUSTER
    APP3 --> REDIS_CLUSTER
    
    APP1 --> KAFKA_CLUSTER
    APP2 --> KAFKA_CLUSTER
    APP3 --> KAFKA_CLUSTER
    
    APP1 --> PROMETHEUS
    APP2 --> PROMETHEUS
    APP3 --> PROMETHEUS
    
    PROMETHEUS --> GRAFANA
    PROMETHEUS --> ALERTMANAGER
    
    APP1 --> ELK_STACK
    APP2 --> ELK_STACK
    APP3 --> ELK_STACK
    
    STANDBY1 -.-> PG_REPLICA1
    STANDBY2 -.-> PG_REPLICA2
    
    COUNTERPARTIES <--> LB
    TRADING_SYSTEMS <--> LB
    ADMIN_CONSOLE <--> GRAFANA
    
    classDef loadbalancer fill:#FF6B6B,stroke:#CC0000,stroke-width:2px
    classDef application fill:#4ECDC4,stroke:#26A69A,stroke-width:2px
    classDef standby fill:#FFE66D,stroke:#FFC107,stroke-width:2px
    classDef storage fill:#95A5A6,stroke:#7F8C8D,stroke-width:2px
    classDef cache fill:#E17055,stroke:#D63031,stroke-width:2px
    classDef monitoring fill:#6C5CE7,stroke:#5F3DC4,stroke-width:2px
    classDef external fill:#FD79A8,stroke:#E84393,stroke-width:2px
    
    class LB,HEALTH_CHECK loadbalancer
    class APP1,APP2,APP3 application
    class STANDBY1,STANDBY2 standby
    class PG_PRIMARY,PG_REPLICA1,PG_REPLICA2 storage
    class REDIS_CLUSTER,KAFKA_CLUSTER cache
    class PROMETHEUS,GRAFANA,ALERTMANAGER,ELK_STACK monitoring
    class COUNTERPARTIES,TRADING_SYSTEMS,ADMIN_CONSOLE external
```

### Cloud-Native Deployment (Phase 4)

```mermaid
graph TB
    subgraph "Cloud Infrastructure"
        subgraph "Kubernetes Cluster"
            subgraph "Ingress Layer"
                INGRESS[Ingress Controller]
                SERVICE_MESH[Service Mesh - Istio]
            end
            
            subgraph "Application Pods"
                POD1[PyFixMsg Plus Pod 1]
                POD2[PyFixMsg Plus Pod 2]
                POD3[PyFixMsg Plus Pod 3]
                HPA[Horizontal Pod Autoscaler]
            end
            
            subgraph "Supporting Services"
                CONFIG_SERVICE[Config Service]
                SECRET_MGR[Secret Manager]
                SERVICE_DISCOVERY[Service Discovery]
            end
            
            subgraph "Storage"
                PVC[Persistent Volume Claims]
                STORAGE_CLASS[Storage Classes]
            end
        end
        
        subgraph "Managed Services"
            CLOUD_DB[(Cloud Database)]
            CLOUD_CACHE[(Cloud Cache)]
            CLOUD_QUEUE[Cloud Message Queue]
            CLOUD_STORAGE[Cloud Object Storage]
        end
        
        subgraph "Monitoring & Observability"
            CLOUD_MONITORING[Cloud Monitoring]
            CLOUD_LOGGING[Cloud Logging]
            APM[Application Performance Monitoring]
            DISTRIBUTED_TRACING[Distributed Tracing]
        end
        
        subgraph "Security"
            IAM[Identity & Access Management]
            KMS[Key Management Service]
            NETWORK_POLICIES[Network Policies]
            SECURITY_SCANNING[Security Scanning]
        end
    end
    
    subgraph "External Integrations"
        CI_CD[CI/CD Pipeline]
        REGISTRY[Container Registry]
        HELM_CHARTS[Helm Charts]
        BACKUP_SERVICE[Backup Service]
    end
    
    INGRESS --> SERVICE_MESH
    SERVICE_MESH --> POD1
    SERVICE_MESH --> POD2
    SERVICE_MESH --> POD3
    
    HPA --> POD1
    HPA --> POD2
    HPA --> POD3
    
    POD1 --> CONFIG_SERVICE
    POD2 --> CONFIG_SERVICE
    POD3 --> CONFIG_SERVICE
    
    CONFIG_SERVICE --> SECRET_MGR
    SECRET_MGR --> KMS
    
    POD1 --> CLOUD_DB
    POD2 --> CLOUD_DB
    POD3 --> CLOUD_DB
    
    POD1 --> CLOUD_CACHE
    POD2 --> CLOUD_CACHE
    POD3 --> CLOUD_CACHE
    
    POD1 --> CLOUD_QUEUE
    POD2 --> CLOUD_QUEUE
    POD3 --> CLOUD_QUEUE
    
    PVC --> STORAGE_CLASS
    POD1 --> PVC
    POD2 --> PVC
    POD3 --> PVC
    
    POD1 --> CLOUD_MONITORING
    POD2 --> CLOUD_MONITORING
    POD3 --> CLOUD_MONITORING
    
    CLOUD_MONITORING --> APM
    CLOUD_LOGGING --> DISTRIBUTED_TRACING
    
    IAM --> POD1
    IAM --> POD2
    IAM --> POD3
    
    NETWORK_POLICIES --> SERVICE_MESH
    SECURITY_SCANNING --> POD1
    
    CI_CD --> REGISTRY
    REGISTRY --> POD1
    HELM_CHARTS --> CONFIG_SERVICE
    BACKUP_SERVICE --> CLOUD_STORAGE
    
    classDef ingress fill:#FF9999,stroke:#FF0000,stroke-width:2px
    classDef application fill:#99CCFF,stroke:#0066CC,stroke-width:2px
    classDef supporting fill:#99FF99,stroke:#00CC00,stroke-width:2px
    classDef storage fill:#FFCC99,stroke:#FF6600,stroke-width:2px
    classDef managed fill:#CC99FF,stroke:#6600CC,stroke-width:2px
    classDef monitoring fill:#FFFF99,stroke:#CCCC00,stroke-width:2px
    classDef security fill:#FF99CC,stroke:#CC0066,stroke-width:2px
    classDef external fill:#CCCCCC,stroke:#666666,stroke-width:2px
    
    class INGRESS,SERVICE_MESH ingress
    class POD1,POD2,POD3,HPA application
    class CONFIG_SERVICE,SECRET_MGR,SERVICE_DISCOVERY supporting
    class PVC,STORAGE_CLASS storage
    class CLOUD_DB,CLOUD_CACHE,CLOUD_QUEUE,CLOUD_STORAGE managed
    class CLOUD_MONITORING,CLOUD_LOGGING,APM,DISTRIBUTED_TRACING monitoring
    class IAM,KMS,NETWORK_POLICIES,SECURITY_SCANNING security
    class CI_CD,REGISTRY,HELM_CHARTS,BACKUP_SERVICE external
```

## Phase Evolution Diagrams

### Feature Evolution Across Phases

```mermaid
graph LR
    subgraph "Phase 1: Core Engine"
        P1A[Core FIX Engine]
        P1B[Session Management]
        P1C[Basic Message Stores]
        P1D[Configuration System]
        P1E[CLI Tools]
        
        P1A --> P1B
        P1B --> P1C
        P1C --> P1D
        P1D --> P1E
    end
    
    subgraph "Phase 2: Production Ready"
        P2A[Comprehensive Testing]
        P2B[Performance Optimization]
        P2C[QuickFIX Interoperability]
        P2D[Complete Documentation]
        P2E[CI/CD Pipeline]
        
        P2A --> P2B
        P2B --> P2C
        P2C --> P2D
        P2D --> P2E
    end
    
    subgraph "Phase 3: Enterprise"
        P3A[Multiple DB Backends]
        P3B[High Availability]
        P3C[Advanced Monitoring]
        P3D[Security Hardening]
        P3E[HFT Optimization]
        
        P3A --> P3B
        P3B --> P3C
        P3C --> P3D
        P3D --> P3E
    end
    
    subgraph "Phase 4: Ecosystem"
        P4A[FIX 5.0+ Support]
        P4B[Cloud Deployment]
        P4C[Trading Framework]
        P4D[Market Data Platform]
        P4E[Community Tools]
        
        P4A --> P4B
        P4B --> P4C
        P4C --> P4D
        P4D --> P4E
    end
    
    P1E --> P2A
    P2E --> P3A
    P3E --> P4A
    
    classDef phase1 fill:#e1f5fe,stroke:#01579b,stroke-width:2px
    classDef phase2 fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef phase3 fill:#e8f5e8,stroke:#1b5e20,stroke-width:2px
    classDef phase4 fill:#fff3e0,stroke:#e65100,stroke-width:2px
    
    class P1A,P1B,P1C,P1D,P1E phase1
    class P2A,P2B,P2C,P2D,P2E phase2
    class P3A,P3B,P3C,P3D,P3E phase3
    class P4A,P4B,P4C,P4D,P4E phase4
```

### Architecture Complexity Evolution

```mermaid
graph LR
    subgraph "Phase 1: Simple"
        P1_APP[Application]
        P1_ENGINE[FixEngine]
        P1_STORE[(SQLite)]
        
        P1_APP --> P1_ENGINE
        P1_ENGINE --> P1_STORE
    end
    
    subgraph "Phase 2: Robust"
        P2_APP[Application]
        P2_ENGINE[FixEngine]
        P2_TESTING[Testing Framework]
        P2_MONITORING[Monitoring]
        P2_STORE[(Enhanced Stores)]
        
        P2_APP --> P2_ENGINE
        P2_ENGINE --> P2_STORE
        P2_ENGINE --> P2_MONITORING
        P2_TESTING --> P2_ENGINE
    end
    
    subgraph "Phase 3: Enterprise"
        P3_LB[Load Balancer]
        P3_APP1[App Instance 1]
        P3_APP2[App Instance 2]
        P3_CLUSTER[(DB Cluster)]
        P3_CACHE[(Cache Layer)]
        P3_MONITOR[Enterprise Monitoring]
        
        P3_LB --> P3_APP1
        P3_LB --> P3_APP2
        P3_APP1 --> P3_CLUSTER
        P3_APP2 --> P3_CLUSTER
        P3_APP1 --> P3_CACHE
        P3_APP2 --> P3_CACHE
        P3_APP1 --> P3_MONITOR
        P3_APP2 --> P3_MONITOR
    end
    
    subgraph "Phase 4: Ecosystem"
        P4_CLOUD[Cloud Platform]
        P4_K8S[Kubernetes]
        P4_SERVICES[Microservices]
        P4_TRADING[Trading Platform]
        P4_MARKET[Market Data]
        P4_COMMUNITY[Community Tools]
        
        P4_CLOUD --> P4_K8S
        P4_K8S --> P4_SERVICES
        P4_SERVICES --> P4_TRADING
        P4_SERVICES --> P4_MARKET
        P4_SERVICES --> P4_COMMUNITY
    end
    
    P1_ENGINE -.->|Evolution| P2_ENGINE
    P2_ENGINE -.->|Scaling| P3_APP1
    P3_APP1 -.->|Cloud Migration| P4_SERVICES
    
    classDef phase1 fill:#E8F5E8,stroke:#4CAF50,stroke-width:2px
    classDef phase2 fill:#E3F2FD,stroke:#2196F3,stroke-width:2px
    classDef phase3 fill:#FFF3E0,stroke:#FF9800,stroke-width:2px
    classDef phase4 fill:#F3E5F5,stroke:#9C27B0,stroke-width:2px
    
    class P1_APP,P1_ENGINE,P1_STORE phase1
    class P2_APP,P2_ENGINE,P2_TESTING,P2_MONITORING,P2_STORE phase2
    class P3_LB,P3_APP1,P3_APP2,P3_CLUSTER,P3_CACHE,P3_MONITOR phase3
    class P4_CLOUD,P4_K8S,P4_SERVICES,P4_TRADING,P4_MARKET,P4_COMMUNITY phase4
```

---

## Conclusion

These component diagrams provide a comprehensive visual representation of PyFixMsg Plus architecture across all phases of development. They illustrate:

1. **System Structure**: How components interact and depend on each other
2. **Data Flow**: How messages and data move through the system
3. **Evolution Path**: How the architecture grows from simple to enterprise-scale
4. **Deployment Options**: Various deployment patterns for different use cases
5. **Technology Integration**: How different technologies and frameworks integrate

The diagrams serve as both design documentation and implementation guidance, helping developers understand the system architecture and make informed decisions about component interactions and system evolution.

---

**Document Maintained By:** Architecture Team  
**Review Frequency:** Quarterly or upon major architectural changes  
**Related Documents:** Implementation Roadmap, Phase Implementation Plans  
**Version Control:** All diagrams are version-controlled and updated with code changes
