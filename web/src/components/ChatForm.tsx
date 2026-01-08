import { useState, useRef, useEffect } from "react";
import { submitChat, ChatMessage } from "@/api/workflows";
import { useJobStore } from "@/stores/jobStore";
import { cn } from "@/lib/utils";

interface ChatFormProps {
  className?: string;
}

const SYSTEM_PROMPT = `You are an expert at creating detailed, creative prompts for AI image and video generation models. Your specialty is crafting NSFW and adult-themed prompts with vivid, explicit descriptions that capture the user's vision while being technically precise for optimal AI generation results.

When users describe what they want, help them refine and expand their ideas into comprehensive prompts that include:
- Detailed physical descriptions
- Specific poses, angles, and compositions
- Lighting and atmosphere
- Art style and quality tags
- Technical parameters (aspect ratio, camera angles, etc.)

Be direct, detailed, and unfiltered in your responses.`;

export function ChatForm({ className }: ChatFormProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([
    { role: "system", content: SYSTEM_PROMPT },
  ]);
  const [input, setInput] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const addJob = useJobStore((state) => state.addJob);
  const jobs = useJobStore((state) => state.jobs);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Listen for completed chat jobs to add assistant responses
  useEffect(() => {
    const latestChatJob = jobs
      .filter((j) => j.type === "chat")
      .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())[0];

    if (latestChatJob?.status === "completed" && latestChatJob.output) {
      const response = (latestChatJob.output as { response?: string })
        .response;
      if (response) {
        setMessages((prev) => {
          // Avoid duplicates
          const lastMsg = prev[prev.length - 1];
          if (lastMsg?.role === "assistant" && lastMsg?.content === response) {
            return prev;
          }
          return [...prev, { role: "assistant", content: response }];
        });
      }
    }
  }, [jobs]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!input.trim() || isSubmitting) return;

    const userMessage: ChatMessage = { role: "user", content: input.trim() };
    const newMessages = [...messages, userMessage];
    setMessages(newMessages);
    setInput("");
    setIsSubmitting(true);

    try {
      const response = await submitChat({
        messages: newMessages,
        max_tokens: 512,
        temperature: 0.9,
        top_p: 0.95,
      });

      // Add job to store
      addJob({
        id: response.id,
        type: "chat",
        status: "pending",
        progress: 0,
        stage: "Queued",
        params: { messages: newMessages },
      });
    } catch (error) {
      console.error("Chat submission error:", error);
      alert(
        `Failed to send message: ${error instanceof Error ? error.message : "Unknown error"}`,
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className={cn("flex flex-col h-[calc(100vh-200px)]", className)}>
      {/* Chat header */}
      <div className="mb-4 p-4 bg-card rounded-lg border border-border">
        <h2 className="text-lg font-semibold mb-1">
          Dolphin Mistral Prompt Assistant
        </h2>
        <p className="text-sm text-muted-foreground">
          Expert AI assistant for creating detailed NSFW image/video prompts
        </p>
      </div>

      {/* Messages area */}
      <div className="flex-1 overflow-y-auto space-y-4 mb-4 p-4 bg-card rounded-lg border border-border">
        {messages
          .filter((m) => m.role !== "system")
          .map((message, idx) => (
            <div
              key={idx}
              className={cn(
                "flex",
                message.role === "user" ? "justify-end" : "justify-start",
              )}
            >
              <div
                className={cn(
                  "max-w-[80%] rounded-lg px-4 py-2",
                  message.role === "user"
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted",
                )}
              >
                <div className="text-xs font-medium mb-1 opacity-70">
                  {message.role === "user" ? "You" : "Assistant"}
                </div>
                <div className="text-sm whitespace-pre-wrap">
                  {message.content}
                </div>
              </div>
            </div>
          ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Describe the image or video you want to create..."
          disabled={isSubmitting}
          className="flex-1 px-4 py-3 bg-card border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={isSubmitting || !input.trim()}
          className="px-6 py-3 bg-primary text-primary-foreground rounded-lg font-medium disabled:opacity-50 hover:opacity-90 transition-opacity"
        >
          {isSubmitting ? "Sending..." : "Send"}
        </button>
      </form>
    </div>
  );
}
