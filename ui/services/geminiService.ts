import { GoogleGenAI } from "@google/genai";
import { K8sResource } from '../types';

// IMPORTANT: In a real app, never expose API keys on the client. 
// This should be proxied through a backend.
// For this environment, we assume process.env.API_KEY is available.

const getAiClient = () => {
  if (!process.env.API_KEY) {
    console.error("API_KEY is missing from environment variables.");
    return null;
  }
  return new GoogleGenAI({ apiKey: process.env.API_KEY });
};

export const analyzeIncident = async (resources: K8sResource[], query: string) => {
  const ai = getAiClient();
  if (!ai) return "API Key not configured. Unable to perform analysis.";

  // Format context for the AI
  const resourceSummary = resources.map(r => {
    const errorSegments = r.statusSegments.filter(s => s.status === 'Error' || s.status === 'Warning');
    return `
Resource: ${r.kind}/${r.namespace}/${r.name}
Significant Events:
${errorSegments.map(s => `- [${s.start.toISOString()} to ${s.end.toISOString()}] Status: ${s.status} (${s.message})`).join('\n')}
Recent Audit Logs:
${r.events.slice(-5).map(e => `- ${e.timestamp.toISOString()}: ${e.verb} by ${e.user} - ${e.message}`).join('\n')}
`;
  }).join('\n---\n');

  const prompt = `
You are a Kubernetes Site Reliability Engineer Expert. 
Analyze the following partial audit log summary for a potential incident.
Focus on identifying root causes, cascading failures, or anomalous patterns.

User Query: "${query}"

Context Data:
${resourceSummary}

Provide a concise, markdown-formatted analysis.
`;

  try {
    const response = await ai.models.generateContent({
      model: 'gemini-2.5-flash',
      contents: prompt,
      config: {
        systemInstruction: "You are an expert SRE tool assistant. Be precise, technical, and helpful."
      }
    });

    return response.text;
  } catch (error) {
    console.error("Gemini API Error:", error);
    return "Failed to analyze incident. Please check console for details.";
  }
};
