// Copyright (c) 2025 Lazycat Apps
// Licensed under the MIT License. See LICENSE file in the project root for details.

import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Form,
  Input,
  Button,
  Card,
  Space,
  Typography,
  Select,
  Checkbox,
  Modal,
  Alert,
  Collapse,
  FloatButton,
  App as AntApp,
  Table,
  Tag,
  Progress,
  Dropdown,
  Tabs,
} from 'antd';
import {
  BugOutlined,
  ScanOutlined,
  HistoryOutlined,
  DownloadOutlined,
  EyeOutlined,
  CopyOutlined,
  FullscreenOutlined,
  FullscreenExitOutlined,
  ReloadOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import 'antd/dist/reset.css';
import './App.css';

const { Title, Text } = Typography;
const { Option } = Select;

// Version info
const APP_VERSION = process.env.REACT_APP_VERSION || require('../package.json').version;
const GIT_COMMIT = process.env.REACT_APP_GIT_COMMIT || 'dev';
const GIT_COMMIT_FULL = process.env.REACT_APP_GIT_COMMIT_FULL || 'development';
const GIT_BRANCH = process.env.REACT_APP_GIT_BRANCH || 'local';
const BUILD_TIME = process.env.REACT_APP_BUILD_TIME || 'dev-build';

// Backend API URL
const BACKEND_API_URL = window._env_?.BACKEND_API_URL || '';

// Debug logger utility
const debugLog = (category, ...args) => {
  const timestamp = new Date().toISOString();
  const message = `[${timestamp}] [${category}]`;
  console.log(message, ...args);
  return {
    timestamp,
    category,
    message: args.map(arg =>
      typeof arg === 'object' ? JSON.stringify(arg, null, 2) : String(arg)
    ).join(' ')
  };
};

function AppContent() {
  const { message, modal } = AntApp.useApp();
  const [form] = Form.useForm();

  // Authentication state
  const [authChecking, setAuthChecking] = useState(true);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [oidcEnabled, setOidcEnabled] = useState(false);
  const [userInfo, setUserInfo] = useState(null);

  // System config state
  const [systemConfig, setSystemConfig] = useState({
    enableDockerScan: false
  });

  // Scan form state
  const [loading, setLoading] = useState(false);

  // Scan history state
  const [scanHistory, setScanHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  // Queue state
  const [queueStatus, setQueueStatus] = useState(null);

  // Config management state
  const [configList, setConfigList] = useState([]);
  const [selectedConfig, setSelectedConfig] = useState('');
  const [saveConfigModalVisible, setSaveConfigModalVisible] = useState(false);
  const [configNameInput, setConfigNameInput] = useState('');

  // Docker images state
  const [dockerImagesModalVisible, setDockerImagesModalVisible] = useState(false);
  const [dockerImages, setDockerImages] = useState([]);
  const [dockerImagesLoading, setDockerImagesLoading] = useState(false);

  // Docker containers state
  const [dockerContainers, setDockerContainers] = useState([]);
  const [dockerContainersLoading, setDockerContainersLoading] = useState(false);
  const [dockerActiveTab, setDockerActiveTab] = useState('containers');

  // Logs modal state
  const [logsModalVisible, setLogsModalVisible] = useState(false);
  const [logsModalMaximized, setLogsModalMaximized] = useState(false);
  const [taskStatus, setTaskStatus] = useState(null);
  const [taskLogs, setTaskLogs] = useState([]);

  // Debug state
  const [debugLogs, setDebugLogs] = useState([]);
  const [debugModalVisible, setDebugModalVisible] = useState(false);
  const [debugEnabled, setDebugEnabled] = useState(false);

  // Refs
  const logsEndRef = useRef(null);
  const debugLogsEndRef = useRef(null);
  const eventSourceRef = useRef(null);
  const statusIntervalRef = useRef(null);

  // Enhanced debug logging function
  const addDebugLog = useCallback((category, ...args) => {
    const log = debugLog(category, ...args);
    setDebugLogs(prev => [...prev, log]);
  }, []);

  // Global error handler
  useEffect(() => {
    const handleError = (event) => {
      addDebugLog('ERROR', 'Global error:', event.error?.message || event.message);
    };
    const handleUnhandledRejection = (event) => {
      addDebugLog('ERROR', 'Unhandled rejection:', event.reason);
    };

    window.addEventListener('error', handleError);
    window.addEventListener('unhandledrejection', handleUnhandledRejection);

    addDebugLog('INIT', 'Trivy Web UI initialized', {
      version: APP_VERSION,
      gitCommit: GIT_COMMIT,
      gitBranch: GIT_BRANCH,
      buildTime: BUILD_TIME,
      userAgent: navigator.userAgent,
      backendUrl: BACKEND_API_URL
    });

    return () => {
      window.removeEventListener('error', handleError);
      window.removeEventListener('unhandledrejection', handleUnhandledRejection);
    };
  }, [addDebugLog]);

  // Load system configuration on mount
  useEffect(() => {
    addDebugLog('SYSTEM', 'Loading system configuration');
    fetch(`${BACKEND_API_URL}/api/v1/system/config`)
      .then(res => res.json())
      .then(data => {
        addDebugLog('SYSTEM', 'System config loaded:', data);
        setSystemConfig({
          enableDockerScan: data.enableDockerScan || false
        });
      })
      .catch(err => {
        addDebugLog('ERROR', 'Failed to load system config:', err);
      });
  }, [addDebugLog]);

  // Check authentication status on mount
  useEffect(() => {
    addDebugLog('AUTH', 'Checking authentication status');
    fetch(`${BACKEND_API_URL}/api/v1/auth/userinfo`, {
      credentials: 'include'
    })
      .then(res => res.json())
      .then(data => {
        addDebugLog('AUTH', 'User info:', data);
        setOidcEnabled(data.oidc_enabled || false);
        if (data.authenticated) {
          setIsAuthenticated(true);
          setUserInfo(data);
          addDebugLog('AUTH', 'User is authenticated');
        } else {
          setIsAuthenticated(false);
          addDebugLog('AUTH', `User is not authenticated (OIDC enabled: ${data.oidc_enabled})`);
        }
        setAuthChecking(false);
      })
      .catch(err => {
        addDebugLog('ERROR', 'Failed to check auth:', err);
        setIsAuthenticated(false);
        setOidcEnabled(false);
        setAuthChecking(false);
      });
  }, [addDebugLog]);

  // Load scan history after auth check
  const loadScanHistory = useCallback(async () => {
    if (authChecking) return;
    if (oidcEnabled && !isAuthenticated) return;

    setHistoryLoading(true);
    try {
      addDebugLog('HISTORY', 'Loading scan history');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/scan?page=1&pageSize=20`, {
        credentials: 'include'
      });

      if (response.ok) {
        const data = await response.json();
        addDebugLog('HISTORY', 'Scan history loaded:', data);
        setScanHistory(data.tasks || []);
      } else {
        const error = await response.json();
        addDebugLog('ERROR', 'Failed to load scan history:', error);
      }
    } catch (error) {
      addDebugLog('ERROR', 'Load scan history exception:', error.message);
    } finally {
      setHistoryLoading(false);
    }
  }, [authChecking, oidcEnabled, isAuthenticated, addDebugLog]);

  // Load queue status
  const loadQueueStatus = useCallback(async () => {
    try {
      const response = await fetch(`${BACKEND_API_URL}/api/v1/queue/status`, {
        credentials: 'include'
      });

      if (response.ok) {
        const data = await response.json();
        setQueueStatus(data);
      }
    } catch (error) {
      addDebugLog('ERROR', 'Load queue status exception:', error.message);
    }
  }, [addDebugLog]);

  // Load config list
  const loadConfigList = useCallback(async () => {
    try {
      addDebugLog('CONFIG', 'Loading config list');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/configs`, {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        setConfigList(data.configs || []);
        addDebugLog('CONFIG', 'Config list loaded:', data.configs);
      } else {
        const error = await response.json();
        addDebugLog('ERROR', 'Failed to load config list:', error);
      }
    } catch (error) {
      addDebugLog('ERROR', 'Load config list exception:', error.message);
    }
  }, [addDebugLog]);

  // Load config by name
  const loadConfigByName = useCallback(async (name) => {
    try {
      addDebugLog('CONFIG', 'Loading config:', name);
      const response = await fetch(`${BACKEND_API_URL}/api/v1/config/${encodeURIComponent(name)}`, {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        addDebugLog('CONFIG', 'Config loaded:', data);

        // Set form values from config
        form.setFieldsValue({
          image: data.imagePrefix || '',
          username: data.username ? atob(data.username) : '',
          password: data.password ? atob(data.password) : '',
          tlsVerify: data.tlsVerify !== undefined ? data.tlsVerify : true,
          severity: data.severity || [],
          ignoreUnfixed: data.ignoreUnfixed || false,
          scanners: data.scanners || ['vuln'],
          detectionPriority: data.detectionPriority || 'precise',
          pkgTypes: data.pkgTypes || [],
          format: data.format || 'json',
        });

        message.success(`已加载配置: ${name}`);
      } else {
        const error = await response.json();
        message.error(`加载配置失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Failed to load config:', error);
      }
    } catch (error) {
      message.error(`加载配置失败: ${error.message}`);
      addDebugLog('ERROR', 'Load config exception:', error.message);
    }
  }, [form, message, addDebugLog]);

  // Load last used config
  const loadLastUsedConfig = useCallback(async () => {
    try {
      addDebugLog('CONFIG', 'Loading last used config');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/config/last-used`, {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        if (data.name) {
          addDebugLog('CONFIG', 'Last used config name:', data.name);
          setSelectedConfig(data.name);
          await loadConfigByName(data.name);
        } else {
          addDebugLog('CONFIG', 'No last used config found');
        }
      } else {
        addDebugLog('ERROR', 'Failed to load last used config');
      }
    } catch (error) {
      addDebugLog('ERROR', 'Load last used config exception:', error.message);
    }
  }, [addDebugLog, loadConfigByName]);

  // Save configuration
  const handleSaveConfig = async () => {
    const name = configNameInput.trim();
    if (!name) {
      message.error('请输入配置名称');
      return;
    }

    // Validate config name
    if (!/^[a-zA-Z0-9._-]+$/.test(name)) {
      message.error('配置名称只能包含字母、数字、点、横线和下划线');
      return;
    }

    try {
      const values = form.getFieldsValue();
      addDebugLog('CONFIG', 'Saving config:', name, { ...values, password: values.password ? '***' : '' });

      // Encode credentials with base64
      const config = {
        imagePrefix: values.image || '',
        username: values.username ? btoa(values.username) : '',
        password: values.password ? btoa(values.password) : '',
        tlsVerify: values.tlsVerify !== undefined ? values.tlsVerify : true,
        severity: values.severity || [],
        ignoreUnfixed: values.ignoreUnfixed || false,
        scanners: values.scanners || ['vuln'],
        detectionPriority: values.detectionPriority || 'precise',
        pkgTypes: values.pkgTypes || [],
        format: values.format || 'json',
      };

      const response = await fetch(`${BACKEND_API_URL}/api/v1/config/${encodeURIComponent(name)}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(config),
      });

      if (response.ok) {
        message.success(`配置已保存: ${name}`);
        addDebugLog('CONFIG', 'Config saved successfully:', name);
        setSaveConfigModalVisible(false);
        setSelectedConfig(name);
        await loadConfigList();
      } else {
        const error = await response.json();
        message.error(`保存失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Failed to save config:', error);
      }
    } catch (error) {
      message.error(`保存失败: ${error.message}`);
      addDebugLog('ERROR', 'Save config exception:', error.message);
    }
  };

  // Delete configuration
  const handleDeleteConfig = async () => {
    if (!selectedConfig) {
      message.warning('请先选择要删除的配置');
      return;
    }

    try {
      const response = await fetch(`${BACKEND_API_URL}/api/v1/config/${encodeURIComponent(selectedConfig)}`, {
        method: 'DELETE',
        credentials: 'include',
      });

      if (response.ok) {
        message.success(`已删除配置: ${selectedConfig}`);
        addDebugLog('CONFIG', 'Config deleted successfully:', selectedConfig);
        setSelectedConfig('');
        await loadConfigList();
      } else {
        const error = await response.json();
        message.error(`删除失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Failed to delete config:', error);
      }
    } catch (error) {
      message.error(`删除失败: ${error.message}`);
      addDebugLog('ERROR', 'Delete config exception:', error.message);
    }
  };

  // Load Docker images from host
  const loadDockerImages = useCallback(async () => {
    setDockerImagesLoading(true);
    try {
      addDebugLog('DOCKER', 'Loading Docker images from host');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/docker/images`, {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        addDebugLog('DOCKER', 'Docker images loaded:', data);
        setDockerImages(data.images || []);
      } else {
        const error = await response.json();
        message.error(`加载镜像列表失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Failed to load Docker images:', error);
      }
    } catch (error) {
      message.error(`加载镜像列表失败: ${error.message}`);
      addDebugLog('ERROR', 'Load Docker images exception:', error.message);
    } finally {
      setDockerImagesLoading(false);
    }
  }, [message, addDebugLog]);

  // Load Docker containers from host
  const loadDockerContainers = useCallback(async () => {
    setDockerContainersLoading(true);
    try {
      addDebugLog('DOCKER', 'Loading Docker containers from host');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/docker/containers`, {
        credentials: 'include',
      });

      if (response.ok) {
        const data = await response.json();
        addDebugLog('DOCKER', 'Docker containers loaded:', data);
        setDockerContainers(data.containers || []);
      } else {
        const error = await response.json();
        message.error(`加载容器列表失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Failed to load Docker containers:', error);
      }
    } catch (error) {
      message.error(`加载容器列表失败: ${error.message}`);
      addDebugLog('ERROR', 'Load Docker containers exception:', error.message);
    } finally {
      setDockerContainersLoading(false);
    }
  }, [message, addDebugLog]);

  // Open Docker selection modal
  const openDockerModal = useCallback(async () => {
    setDockerImagesModalVisible(true);
    setDockerActiveTab('containers');
    await loadDockerContainers();
    await loadDockerImages();
  }, [loadDockerContainers, loadDockerImages]);

  // Select Docker image
  const handleSelectDockerImage = useCallback((fullName) => {
    addDebugLog('DOCKER', 'Selecting Docker image:', fullName);
    form.setFieldsValue({ image: fullName });
    setDockerImagesModalVisible(false);
    message.success(`已选择镜像: ${fullName}`);
  }, [form, message, addDebugLog]);

  // Select container image
  const handleSelectContainerImage = useCallback((imageName) => {
    addDebugLog('DOCKER', 'Selecting container image:', imageName);
    form.setFieldsValue({ image: imageName });
    setDockerImagesModalVisible(false);
    message.success(`已选择容器镜像: ${imageName}`);
  }, [form, message, addDebugLog]);

  // Load data after auth check
  useEffect(() => {
    if (!authChecking && (!oidcEnabled || isAuthenticated)) {
      loadScanHistory();
      loadQueueStatus();
      // Load config list first, then load last used config
      loadConfigList().then(() => {
        loadLastUsedConfig();
      });
    }
  }, [authChecking, oidcEnabled, isAuthenticated, loadScanHistory, loadQueueStatus, loadConfigList, loadLastUsedConfig]);

  // Auto-scroll logs
  useEffect(() => {
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [taskLogs]);

  // Auto-scroll debug logs
  useEffect(() => {
    if (debugLogsEndRef.current) {
      debugLogsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [debugLogs]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (statusIntervalRef.current) {
        clearInterval(statusIntervalRef.current);
      }
    };
  }, []);

  // Start log stream for a task
  const startLogStream = useCallback((taskId) => {
    addDebugLog('LOG_STREAM', 'Starting log stream for task:', taskId);

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }
    if (statusIntervalRef.current) {
      clearInterval(statusIntervalRef.current);
    }

    setTaskLogs([]);
    const url = `${BACKEND_API_URL}/api/v1/scan/${taskId}/logs`;
    addDebugLog('LOG_STREAM', 'Creating EventSource:', url);

    const eventSource = new EventSource(url);
    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      addDebugLog('LOG_STREAM', 'EventSource connection opened');
    };

    // Poll status to update task state
    statusIntervalRef.current = setInterval(async () => {
      try {
        const statusUrl = `${BACKEND_API_URL}/api/v1/scan/${taskId}`;
        const response = await fetch(statusUrl, { credentials: 'include' });
        if (response.ok) {
          const data = await response.json();
          setTaskStatus(data);
          addDebugLog('LOG_STREAM', 'Status update:', data.status);
          if (data.status === 'completed' || data.status === 'failed') {
            addDebugLog('LOG_STREAM', 'Task finished, stopping polling');
            clearInterval(statusIntervalRef.current);
            statusIntervalRef.current = null;
            if (eventSourceRef.current) {
              eventSourceRef.current.close();
            }
            // Reload history
            loadScanHistory();
          }
        }
      } catch (error) {
        addDebugLog('ERROR', 'Status polling failed:', error.message);
      }
    }, 1000);

    eventSource.onmessage = (event) => {
      setTaskLogs(prev => [...prev, event.data]);
    };

    eventSource.onerror = (error) => {
      addDebugLog('ERROR', 'EventSource error:', error);
      eventSource.close();
    };
  }, [addDebugLog, loadScanHistory]);

  // Submit scan form
  const onFinish = async (values) => {
    addDebugLog('SCAN', 'Starting scan task with values:', { ...values, password: values.password ? '***' : '' });
    setLoading(true);

    try {
      const url = `${BACKEND_API_URL}/api/v1/scan`;
      addDebugLog('SCAN', 'Sending scan request to:', url);

      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(values),
      });

      addDebugLog('SCAN', 'Scan response received:', { status: response.status, ok: response.ok });

      if (response.ok) {
        const data = await response.json();
        addDebugLog('SCAN', 'Scan task created:', data);
        message.success('扫描任务已创建！');
        setLogsModalVisible(true);
        startLogStream(data.id);
        // Reload queue status
        loadQueueStatus();
      } else {
        const error = await response.json();
        addDebugLog('ERROR', 'Scan failed:', error);
        message.error(`创建扫描任务失败: ${error.error || '未知错误'}`);
      }
    } catch (error) {
      addDebugLog('ERROR', 'Scan exception:', error.message);
      message.error(`请求失败: ${error.message}`);
    } finally {
      setLoading(false);
    }
  };

  // Handle login
  const handleLogin = () => {
    window.location.href = `${BACKEND_API_URL}/api/v1/auth/login`;
  };

  // Handle logout
  const handleLogout = async () => {
    try {
      await fetch(`${BACKEND_API_URL}/api/v1/auth/logout`, {
        method: 'POST',
        credentials: 'include'
      });
      setIsAuthenticated(false);
      setUserInfo(null);
      message.success('已退出登录');
    } catch (err) {
      message.error('退出登录失败');
    }
  };

  // Close logs modal
  const handleCloseLogsModal = () => {
    setLogsModalVisible(false);
    setLogsModalMaximized(false);
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }
    if (statusIntervalRef.current) {
      clearInterval(statusIntervalRef.current);
      statusIntervalRef.current = null;
    }
  };

  // View logs for a task
  const handleViewLogs = (taskId) => {
    setLogsModalVisible(true);
    startLogStream(taskId);
  };

  // Download report
  const handleDownloadReport = (taskId, format = 'json') => {
    addDebugLog('DOWNLOAD', 'Downloading report:', taskId, format);
    const url = `${BACKEND_API_URL}/api/v1/scan/${taskId}/report/${format}`;
    window.open(url, '_blank');
  };

  // Delete scan task
  const handleDeleteTask = async (taskId) => {
    console.log('handleDeleteTask called with taskId:', taskId);
    try {
      addDebugLog('DELETE', 'Deleting task:', taskId);
      const url = `${BACKEND_API_URL}/api/v1/scan/${taskId}`;
      console.log('Sending DELETE request to:', url);
      const response = await fetch(url, {
        method: 'DELETE',
        credentials: 'include',
      });

      console.log('DELETE response status:', response.status);
      if (response.ok) {
        message.success('删除成功');
        addDebugLog('DELETE', 'Task deleted successfully:', taskId);
        loadScanHistory(); // Reload history
      } else {
        const error = await response.json();
        message.error(`删除失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Delete failed:', error);
      }
    } catch (error) {
      console.error('Delete task error:', error);
      message.error(`删除失败: ${error.message}`);
      addDebugLog('ERROR', 'Delete exception:', error.message);
    }
  };

  // Delete all scan tasks
  const handleDeleteAllTasks = async () => {
    try {
      addDebugLog('DELETE_ALL', 'Deleting all tasks');
      const response = await fetch(`${BACKEND_API_URL}/api/v1/scan`, {
        method: 'DELETE',
        credentials: 'include',
      });

      if (response.ok) {
        message.success('已清空所有扫描历史');
        addDebugLog('DELETE_ALL', 'All tasks deleted successfully');
        loadScanHistory(); // Reload history
      } else {
        const error = await response.json();
        message.error(`清空失败: ${error.error || '未知错误'}`);
        addDebugLog('ERROR', 'Delete all failed:', error);
      }
    } catch (error) {
      message.error(`清空失败: ${error.message}`);
      addDebugLog('ERROR', 'Delete all exception:', error.message);
    }
  };

  // Show loading while checking auth
  if (authChecking) {
    return (
      <div className="App">
        <div className="container">
          <Card style={{ textAlign: 'center', padding: '50px' }}>
            <Title level={3}>正在检查登录状态...</Title>
          </Card>
        </div>
      </div>
    );
  }

  // Show login page if OIDC is enabled and user is not authenticated
  if (oidcEnabled && !isAuthenticated) {
    return (
      <div className="App">
        <div className="container">
          <Title level={2}>Trivy Web UI - 容器镜像安全扫描</Title>
          <Card style={{ textAlign: 'center', padding: '50px' }}>
            <Title level={3}>请登录后使用</Title>
            <Text type="secondary" style={{ display: 'block', marginBottom: '24px', fontSize: '16px' }}>
              登录后您可以扫描镜像并查看漏洞报告
            </Text>
            <Button type="primary" size="large" onClick={handleLogin}>
              登录
            </Button>
          </Card>
          <div className="footer">
            <Text type="secondary">
              Trivy Web UI · v{APP_VERSION}
              {GIT_COMMIT !== 'dev' && GIT_COMMIT_FULL !== 'development' && (
                <>
                  {' · '}
                  <a
                    href={`https://github.com/lazycatapps/trivy/commit/${GIT_COMMIT_FULL}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    title={`Commit: ${GIT_COMMIT_FULL}`}
                  >
                    {GIT_COMMIT}
                  </a>
                </>
              )}
              {' · '}
              Copyright © {new Date().getFullYear()} Lazycat Apps
              {' · '}
              <a href="https://github.com/lazycatapps/trivy" target="_blank" rel="noopener noreferrer">
                GitHub
              </a>
            </Text>
          </div>
        </div>
      </div>
    );
  }

  // Main application UI
  return (
    <div className="App">
      <div className="container">
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <Title level={2} style={{ margin: 0 }}>Trivy Web UI - 容器镜像安全扫描</Title>
          {oidcEnabled && (
            <Space>
              {userInfo && (
                <Text type="secondary">
                  {userInfo.email}
                  {userInfo.is_admin && <span style={{ color: '#1890ff', marginLeft: '8px' }}>(管理员)</span>}
                </Text>
              )}
              <Button onClick={handleLogout}>退出登录</Button>
            </Space>
          )}
        </div>

        {/* Queue Status */}
        {queueStatus && queueStatus.queueLength > 0 && (
          <Alert
            message={`队列中有 ${queueStatus.queueLength} 个任务等待扫描`}
            description={`预计平均等待时间: ${Math.round(queueStatus.averageWaitTime || 0)} 秒`}
            type="info"
            showIcon
            style={{ marginBottom: '16px' }}
          />
        )}

        {/* Scan Form */}
        <Card title={<><ScanOutlined /> 扫描配置</>} style={{ marginBottom: '24px' }}>
          <Form
            form={form}
            layout="vertical"
            onFinish={onFinish}
            initialValues={{
              image: '',
              tlsVerify: true,
              severity: [],
              ignoreUnfixed: false,
              scanners: ['vuln'],
              detectionPriority: 'precise',
              pkgTypes: [],
              format: 'json',
            }}
          >
            {/* Image Information */}
            <Form.Item
              label="镜像地址"
              name="image"
              rules={[{ required: true, message: '请输入镜像地址' }]}
            >
              <Input
                placeholder="例如: docker.io/library/nginx:latest"
                addonAfter={
                  systemConfig.enableDockerScan && (
                    <Button
                      type="text"
                      icon={<ScanOutlined />}
                      onClick={openDockerModal}
                      loading={dockerImagesLoading || dockerContainersLoading}
                      title="从宿主机选择镜像或容器"
                      style={{ margin: -5, padding: '0 8px' }}
                    >
                      从宿主机选择
                    </Button>
                  )
                }
              />
            </Form.Item>

            <Space direction="horizontal" style={{ width: '100%' }} size="large">
              <Form.Item
                label="仓库用户名"
                name="username"
                style={{ flex: 1, minWidth: 200 }}
              >
                <Input placeholder="选填（私有仓库需要）" />
              </Form.Item>

              <Form.Item
                label="仓库密码"
                name="password"
                style={{ flex: 1, minWidth: 200 }}
              >
                <Input.Password placeholder="选填（私有仓库需要）" />
              </Form.Item>

              <Form.Item
                label=" "
                name="tlsVerify"
                valuePropName="checked"
                style={{ flex: 1, minWidth: 200 }}
              >
                <Checkbox>启用 TLS 证书验证</Checkbox>
              </Form.Item>
            </Space>

            {/* Scan Options */}
            <Collapse
              items={[
                {
                  key: '1',
                  label: '扫描选项',
                  children: (
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Form.Item
                        label="漏洞严重等级"
                        name="severity"
                      >
                        <Select
                          mode="multiple"
                          placeholder="选择要显示的漏洞等级（默认全部）"
                          allowClear
                        >
                          <Option value="CRITICAL">CRITICAL</Option>
                          <Option value="HIGH">HIGH</Option>
                          <Option value="MEDIUM">MEDIUM</Option>
                          <Option value="LOW">LOW</Option>
                          <Option value="UNKNOWN">UNKNOWN</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="ignoreUnfixed"
                        valuePropName="checked"
                      >
                        <Checkbox>只显示有修复方案的漏洞</Checkbox>
                      </Form.Item>

                      <Form.Item
                        label="扫描器类型"
                        name="scanners"
                      >
                        <Select
                          mode="multiple"
                          placeholder="选择扫描类型"
                        >
                          <Option value="vuln">漏洞扫描</Option>
                          <Option value="misconfig">配置错误扫描</Option>
                          <Option value="secret">密钥泄露扫描</Option>
                          <Option value="license">许可证扫描</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        label="检测优先级"
                        name="detectionPriority"
                      >
                        <Select>
                          <Option value="precise">精确模式（减少误报）</Option>
                          <Option value="comprehensive">全面模式（更多检测）</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        label="包类型"
                        name="pkgTypes"
                      >
                        <Select
                          mode="multiple"
                          placeholder="选择要扫描的包类型（默认全部）"
                          allowClear
                        >
                          <Option value="os">操作系统包</Option>
                          <Option value="library">应用依赖包</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        label="输出格式"
                        name="format"
                      >
                        <Select>
                          <Option value="json">JSON</Option>
                          <Option value="table">Table</Option>
                          <Option value="sarif">SARIF</Option>
                          <Option value="cyclonedx">CycloneDX SBOM</Option>
                          <Option value="spdx">SPDX SBOM</Option>
                        </Select>
                      </Form.Item>
                    </Space>
                  ),
                },
              ]}
            />

            {/* Advanced Options */}
            <Collapse
              style={{ marginBottom: '16px' }}
              items={[
                {
                  key: 'advanced',
                  label: '高级选项',
                  children: (
                    <Space direction="vertical" style={{ width: '100%' }}>
                      <Alert
                        message="配置管理说明"
                        description="您可以保存多份扫描配置，并通过下拉框快速切换。配置信息会保存在服务器端。"
                        type="info"
                        showIcon
                      />
                      <Space wrap>
                        <Text>选择配置：</Text>
                        <Select
                          placeholder="选择已保存的配置"
                          style={{ width: 250 }}
                          value={selectedConfig || undefined}
                          onChange={(value) => {
                            setSelectedConfig(value);
                            loadConfigByName(value);
                          }}
                          showSearch
                          allowClear
                        >
                          {configList.map(name => (
                            <Option key={name} value={name}>
                              {name}
                            </Option>
                          ))}
                        </Select>
                        <Button
                          type="primary"
                          onClick={() => {
                            setConfigNameInput(selectedConfig || '');
                            setSaveConfigModalVisible(true);
                          }}
                        >
                          保存当前配置
                        </Button>
                        <Button
                          danger
                          onClick={handleDeleteConfig}
                          disabled={!selectedConfig}
                        >
                          删除选中配置
                        </Button>
                      </Space>
                      <div style={{ marginTop: '16px', paddingTop: '16px', borderTop: '1px solid #f0f0f0' }}>
                        <Checkbox
                          checked={debugEnabled}
                          onChange={(e) => setDebugEnabled(e.target.checked)}
                        >
                          启用调试模式（显示调试按钮）
                        </Checkbox>
                      </div>
                    </Space>
                  ),
                },
              ]}
            />

            <Form.Item style={{ marginTop: '24px' }}>
              <Button type="primary" htmlType="submit" loading={loading} size="large" block icon={<ScanOutlined />}>
                开始扫描
              </Button>
            </Form.Item>
          </Form>
        </Card>

        {/* Scan History */}
        <Card
          title={<><HistoryOutlined /> 扫描历史</>}
          loading={historyLoading}
          extra={
            scanHistory.length > 0 && (
              <Button
                danger
                size="small"
                onClick={() => {
                  modal.confirm({
                    title: '确认清空',
                    content: `确定要清空所有扫描历史吗？此操作无法撤销。`,
                    okText: '清空',
                    okType: 'danger',
                    cancelText: '取消',
                    onOk() {
                      return new Promise((resolve, reject) => {
                        handleDeleteAllTasks()
                          .then(() => resolve())
                          .catch((err) => reject(err));
                      });
                    },
                  });
                }}
              >
                清空所有
              </Button>
            )
          }
        >
          <Table
            dataSource={scanHistory}
            rowKey="id"
            pagination={{ pageSize: 10 }}
            columns={[
              {
                title: '镜像名称',
                dataIndex: 'image',
                key: 'image',
                render: (image) => (
                  <Space>
                    <Text>{image}</Text>
                    <Button
                      type="text"
                      size="small"
                      icon={<CopyOutlined />}
                      onClick={() => {
                        navigator.clipboard.writeText(image);
                        message.success('镜像地址已复制到剪贴板');
                      }}
                    />
                  </Space>
                ),
              },
              {
                title: '扫描时间',
                dataIndex: 'startTime',
                key: 'startTime',
                render: (time) => time ? new Date(time).toLocaleString('zh-CN') : '-',
              },
              {
                title: '状态',
                dataIndex: 'status',
                key: 'status',
                render: (status, record) => {
                  const statusMap = {
                    queued: { color: 'default', text: '队列中' },
                    running: { color: 'processing', text: '扫描中' },
                    completed: { color: 'success', text: '已完成' },
                    failed: { color: 'error', text: '失败' },
                  };
                  const config = statusMap[status] || { color: 'default', text: status };
                  return (
                    <Space direction="vertical" size="small">
                      <Tag color={config.color}>{config.text}</Tag>
                      {status === 'queued' && record.queuePosition && (
                        <Text type="secondary" style={{ fontSize: '12px' }}>
                          前面还有 {record.queuePosition} 个任务
                        </Text>
                      )}
                    </Space>
                  );
                },
              },
              {
                title: '漏洞统计',
                dataIndex: 'summary',
                key: 'summary',
                render: (summary) => {
                  if (!summary) return '-';
                  return (
                    <Space size={[0, 4]} wrap>
                      {summary.critical > 0 && <Tag color="red">CRITICAL: {summary.critical}</Tag>}
                      {summary.high > 0 && <Tag color="orange">HIGH: {summary.high}</Tag>}
                      {summary.medium > 0 && <Tag color="yellow">MEDIUM: {summary.medium}</Tag>}
                      {summary.low > 0 && <Tag color="blue">LOW: {summary.low}</Tag>}
                    </Space>
                  );
                },
              },
              {
                title: '操作',
                key: 'actions',
                render: (_, record) => (
                  <Space size="small">
                    <Button
                      size="small"
                      icon={<EyeOutlined />}
                      onClick={() => handleViewLogs(record.id)}
                    >
                      查看日志
                    </Button>
                    {record.status === 'completed' && (
                      <Dropdown
                        menu={{
                          items: [
                            { key: 'json', label: 'JSON', onClick: () => handleDownloadReport(record.id, 'json') },
                            { key: 'sarif', label: 'SARIF', onClick: () => handleDownloadReport(record.id, 'sarif') },
                            { key: 'cyclonedx', label: 'CycloneDX', onClick: () => handleDownloadReport(record.id, 'cyclonedx') },
                            { key: 'spdx', label: 'SPDX', onClick: () => handleDownloadReport(record.id, 'spdx') },
                            { key: 'table', label: 'Table', onClick: () => handleDownloadReport(record.id, 'table') },
                          ],
                        }}
                      >
                        <Button
                          size="small"
                          icon={<DownloadOutlined />}
                          type="primary"
                        >
                          下载报告
                        </Button>
                      </Dropdown>
                    )}
                    <Button
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={(e) => {
                        e.stopPropagation();
                        console.log('Delete button clicked for task:', record.id);
                        modal.confirm({
                          title: '确认删除',
                          content: `确定要删除这条扫描记录吗？此操作无法撤销。`,
                          okText: '删除',
                          okType: 'danger',
                          cancelText: '取消',
                          onOk() {
                            console.log('Modal OK clicked, calling handleDeleteTask');
                            return new Promise((resolve, reject) => {
                              handleDeleteTask(record.id)
                                .then(() => {
                                  console.log('handleDeleteTask completed successfully');
                                  resolve();
                                })
                                .catch((err) => {
                                  console.error('handleDeleteTask failed:', err);
                                  reject(err);
                                });
                            });
                          },
                          onCancel() {
                            console.log('Modal cancelled');
                          },
                        });
                      }}
                    >
                      删除
                    </Button>
                  </Space>
                ),
              },
            ]}
          />
        </Card>

        {/* Footer */}
        <div className="footer">
          <Text type="secondary">
            Trivy Web UI · v{APP_VERSION}
            {GIT_COMMIT !== 'dev' && GIT_COMMIT_FULL !== 'development' && (
              <>
                {' · '}
                <a
                  href={`https://github.com/lazycatapps/trivy/commit/${GIT_COMMIT_FULL}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  title={`Commit: ${GIT_COMMIT_FULL}`}
                >
                  {GIT_COMMIT}
                </a>
              </>
            )}
            {' · '}
            Copyright © {new Date().getFullYear()} Lazycat Apps
            {' · '}
            <a href="https://github.com/lazycatapps/trivy" target="_blank" rel="noopener noreferrer">
              GitHub
            </a>
          </Text>
        </div>

        {/* Logs Modal */}
        <Modal
          title={
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>扫描日志</span>
              <div>
                <Button
                  type="text"
                  icon={<CopyOutlined />}
                  onClick={() => {
                    const logsText = taskLogs.join('\n');
                    navigator.clipboard.writeText(logsText).then(() => {
                      message.success('日志已复制到剪贴板');
                      addDebugLog('UI', 'Logs copied to clipboard');
                    }).catch(err => {
                      message.error('复制失败');
                      addDebugLog('ERROR', 'Failed to copy logs:', err);
                    });
                  }}
                  style={{ marginRight: '8px' }}
                >
                  复制
                </Button>
                <Button
                  type="text"
                  icon={logsModalMaximized ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
                  onClick={() => setLogsModalMaximized(!logsModalMaximized)}
                  style={{ marginRight: '24px' }}
                >
                  {logsModalMaximized ? '恢复' : '最大化'}
                </Button>
              </div>
            </div>
          }
          open={logsModalVisible}
          onCancel={handleCloseLogsModal}
          footer={[
            <Button key="close" onClick={handleCloseLogsModal}>
              关闭
            </Button>
          ]}
          width={logsModalMaximized ? '95vw' : 900}
          style={logsModalMaximized ? { top: 10, paddingBottom: 0 } : {}}
        >
          {taskStatus && (
            <>
              <Alert
                message={`任务状态: ${taskStatus.status}`}
                type={taskStatus.status === 'completed' ? 'success' : taskStatus.status === 'failed' ? 'error' : 'info'}
                style={{ marginBottom: '16px' }}
              />
              {taskStatus.status === 'queued' && taskStatus.queuePosition && (
                <Progress
                  percent={0}
                  format={() => `队列中，前面还有 ${taskStatus.queuePosition} 个任务`}
                  style={{ marginBottom: '16px' }}
                />
              )}
            </>
          )}
          <div style={{
            background: '#000',
            color: '#0f0',
            padding: '16px',
            borderRadius: '4px',
            fontFamily: 'monospace',
            fontSize: '12px',
            maxHeight: logsModalMaximized ? 'calc(100vh - 180px)' : '500px',
            overflowY: 'auto'
          }}>
            {taskLogs.map((log, index) => {
              // Try to detect and format JSON
              let displayLog = log;
              try {
                // Check if log contains JSON (starts with { or [)
                const trimmed = log.trim();
                if ((trimmed.startsWith('{') && trimmed.endsWith('}')) ||
                    (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
                  const parsed = JSON.parse(trimmed);
                  displayLog = JSON.stringify(parsed, null, 2);
                }
              } catch (e) {
                // Not JSON, keep original
              }

              return (
                <div key={index} style={{ marginBottom: '4px', whiteSpace: 'pre-wrap' }}>
                  {displayLog}
                </div>
              );
            })}
            <div ref={logsEndRef} />
          </div>
        </Modal>

        {/* Debug Modal */}
        <Modal
          title="调试日志"
          open={debugModalVisible}
          onCancel={() => setDebugModalVisible(false)}
          footer={[
            <Button key="clear" onClick={() => setDebugLogs([])}>
              清空日志
            </Button>,
            <Button key="close" onClick={() => setDebugModalVisible(false)}>
              关闭
            </Button>
          ]}
          width={900}
        >
          <Alert
            message="调试信息"
            description={`总计 ${debugLogs.length} 条日志记录`}
            type="info"
            style={{ marginBottom: '16px' }}
          />
          <div style={{
            background: '#1e1e1e',
            color: '#d4d4d4',
            padding: '16px',
            borderRadius: '4px',
            fontFamily: 'Consolas, Monaco, "Courier New", monospace',
            fontSize: '12px',
            maxHeight: '600px',
            overflowY: 'auto'
          }}>
            {debugLogs.map((log, index) => (
              <div key={index} style={{ marginBottom: '8px', borderBottom: '1px solid #333', paddingBottom: '4px' }}>
                <div style={{ color: '#569cd6' }}>
                  [{log.timestamp}] <span style={{ color: '#4ec9b0' }}>[{log.category}]</span>
                </div>
                <div style={{ color: '#ce9178', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                  {log.message}
                </div>
              </div>
            ))}
            <div ref={debugLogsEndRef} />
          </div>
        </Modal>

        {/* Save Config Modal */}
        <Modal
          title="保存配置"
          open={saveConfigModalVisible}
          onOk={handleSaveConfig}
          onCancel={() => setSaveConfigModalVisible(false)}
          okText="保存"
          cancelText="取消"
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              message="配置名称规则"
              description="只能包含字母、数字、点、横线和下划线，例如：default、prod-env、my.config"
              type="info"
              showIcon
              style={{ marginBottom: '16px' }}
            />
            <Text>配置名称：</Text>
            <Input
              placeholder="输入配置名称"
              value={configNameInput}
              onChange={(e) => setConfigNameInput(e.target.value)}
              onPressEnter={handleSaveConfig}
            />
            {configList.includes(configNameInput.trim()) && (
              <Alert
                message="配置名称已存在，保存将会覆盖原配置"
                type="warning"
                showIcon
              />
            )}
          </Space>
        </Modal>

        {/* Docker Images/Containers Modal */}
        <Modal
          title={
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginRight: '48px' }}>
              <span>从宿主机选择镜像或容器</span>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  if (dockerActiveTab === 'containers') {
                    loadDockerContainers();
                  } else {
                    loadDockerImages();
                  }
                }}
                loading={dockerActiveTab === 'containers' ? dockerContainersLoading : dockerImagesLoading}
              >
                刷新
              </Button>
            </div>
          }
          open={dockerImagesModalVisible}
          onCancel={() => setDockerImagesModalVisible(false)}
          footer={[
            <Button key="close" onClick={() => setDockerImagesModalVisible(false)}>
              关闭
            </Button>
          ]}
          width={1000}
        >
          <Tabs
            activeKey={dockerActiveTab}
            onChange={setDockerActiveTab}
            items={[
              {
                key: 'containers',
                label: `运行中容器 (${dockerContainers.length})`,
                children: (
                  <>
                    <Alert
                      message="提示"
                      description="点击行可直接选择镜像并填充到表单"
                      type="info"
                      showIcon
                      style={{ marginBottom: '16px' }}
                    />
                    <Table
                      dataSource={dockerContainers}
                      rowKey="containerId"
                      pagination={{
                        defaultPageSize: 10,
                        showSizeChanger: true,
                        pageSizeOptions: ['10', '20', '50', '100'],
                        showTotal: (total, range) => `${range[0]}-${range[1]} / 共 ${total} 条`,
                      }}
                      loading={dockerContainersLoading}
                      scroll={{ x: 1200 }}
                      onRow={(record) => ({
                        onClick: (e) => {
                          // 如果点击的是按钮，不触发行点击
                          if (e.target.closest('button')) return;
                          handleSelectContainerImage(record.image);
                        },
                        style: { cursor: 'pointer' }
                      })}
                      columns={[
                        {
                          title: '容器名称',
                          dataIndex: 'containerName',
                          key: 'containerName',
                          width: 180,
                          sorter: (a, b) => a.containerName.localeCompare(b.containerName),
                          render: (name) => <Text strong>{name}</Text>,
                        },
                        {
                          title: '镜像',
                          dataIndex: 'image',
                          key: 'image',
                          width: 250,
                          sorter: (a, b) => a.image.localeCompare(b.image),
                          render: (image) => (
                            <Space>
                              <Button
                                type="text"
                                size="small"
                                icon={<CopyOutlined />}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  navigator.clipboard.writeText(image);
                                  message.success('镜像地址已复制到剪贴板');
                                }}
                                title="复制镜像地址"
                              />
                              <Text>{image}</Text>
                            </Space>
                          ),
                        },
                        {
                          title: '容器 ID',
                          dataIndex: 'containerId',
                          key: 'containerId',
                          width: 120,
                          sorter: (a, b) => a.containerId.localeCompare(b.containerId),
                          render: (id) => <Text code>{id}</Text>,
                        },
                        {
                          title: '状态',
                          dataIndex: 'status',
                          key: 'status',
                          width: 150,
                          sorter: (a, b) => a.status.localeCompare(b.status),
                          render: (status) => <Tag color="green">{status}</Tag>,
                        },
                        {
                          title: '端口',
                          dataIndex: 'ports',
                          key: 'ports',
                          width: 200,
                          sorter: (a, b) => (a.ports || '').localeCompare(b.ports || ''),
                        },
                        {
                          title: '创建时间',
                          dataIndex: 'created',
                          key: 'created',
                          width: 200,
                          sorter: (a, b) => (a.created || '').localeCompare(b.created || ''),
                        },
                      ]}
                    />
                  </>
                ),
              },
              {
                key: 'images',
                label: `镜像列表 (${dockerImages.length})`,
                children: (
                  <>
                    <Alert
                      message="提示"
                      description="点击行可直接选择镜像并填充到表单"
                      type="info"
                      showIcon
                      style={{ marginBottom: '16px' }}
                    />
                    <Table
                      dataSource={dockerImages}
                      rowKey="imageId"
                      pagination={{
                        defaultPageSize: 10,
                        showSizeChanger: true,
                        pageSizeOptions: ['10', '20', '50', '100'],
                        showTotal: (total, range) => `${range[0]}-${range[1]} / 共 ${total} 条`,
                      }}
                      loading={dockerImagesLoading}
                      scroll={{ x: 1000 }}
                      onRow={(record) => ({
                        onClick: (e) => {
                          // 如果点击的是按钮，不触发行点击
                          if (e.target.closest('button')) return;
                          handleSelectDockerImage(record.fullName);
                        },
                        style: { cursor: 'pointer' }
                      })}
                      columns={[
                        {
                          title: '镜像名称',
                          dataIndex: 'repository',
                          key: 'repository',
                          width: 350,
                          sorter: (a, b) => a.repository.localeCompare(b.repository),
                          render: (repository, record) => (
                            <Space>
                              <Button
                                type="text"
                                size="small"
                                icon={<CopyOutlined />}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  navigator.clipboard.writeText(record.fullName);
                                  message.success('镜像地址已复制到剪贴板');
                                }}
                                title="复制镜像地址"
                              />
                              <Text strong>{repository}</Text>
                              <Tag color="blue">{record.tag}</Tag>
                            </Space>
                          ),
                        },
                        {
                          title: '镜像 ID',
                          dataIndex: 'imageId',
                          key: 'imageId',
                          width: 120,
                          sorter: (a, b) => a.imageId.localeCompare(b.imageId),
                          render: (id) => <Text code>{id}</Text>,
                        },
                        {
                          title: '创建时间',
                          dataIndex: 'created',
                          key: 'created',
                          width: 200,
                          sorter: (a, b) => (a.created || '').localeCompare(b.created || ''),
                        },
                        {
                          title: '大小',
                          dataIndex: 'size',
                          key: 'size',
                          width: 120,
                          sorter: (a, b) => (a.size || '').localeCompare(b.size || ''),
                        },
                      ]}
                    />
                  </>
                ),
              },
            ]}
          />
        </Modal>

        {/* Debug Float Button */}
        {debugEnabled && (
          <FloatButton
            icon={<BugOutlined />}
            type="primary"
            style={{ right: 24, bottom: 24 }}
            onClick={() => setDebugModalVisible(true)}
            badge={{ count: debugLogs.length, overflowCount: 99 }}
            tooltip="查看调试日志"
          />
        )}
      </div>
    </div>
  );
}

function App() {
  return (
    <AntApp>
      <AppContent />
    </AntApp>
  );
}

export default App;
