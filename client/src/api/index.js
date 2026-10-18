import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 60000,
})

export const getDashboard = () => api.get('/dashboard')
export const getWallet = () => api.get('/wallet')
export const generateWallet = () => api.post('/wallet/generate')
export const getTrades = (params) => api.get('/trades', { params })
export const getTrade = (id) => api.get(`/trades/${id}`)
export const getEvents = (params) => api.get('/events', { params })
export const getEvent = (id) => api.get(`/events/${id}`)
export const getAnalysisLogs = (params) => api.get('/analysis', { params })
export const resolveTrade = (id, data) => api.post(`/trades/${id}/resolve`, data)
export const triggerTrading = () => api.post('/trading/run')

export const getAccountProfile = (address, sync = false) =>
  api.get('/account/profile', { params: { address, ...(sync ? { sync: 1 } : {}) } })
export const getAccountActivity = (address, params = {}) =>
  api.get('/account/activity', { params: { address, ...params } })
export const getAccountPositions = (address, params = {}) =>
  api.get('/account/positions', { params: { address, ...params } })

export default api
