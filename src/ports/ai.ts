export interface EmbeddingClient {
  createEmbedding(model: string, input: string): Promise<number[]>;
}

export interface ResponseClient {
  createTextResponse(input: {
    instructions: string;
    model: string;
    text: string;
  }): Promise<string>;
}

export type PlannedToolCall =
  | {
      name: 'complete_task';
      result: string | null;
      taskReference: string;
    }
  | {
      assigneeReference: string | null;
      details: string;
      dueAt: string | null;
      name: 'create_follow_up_task';
      title: string;
    }
  | {
      name: 'list_open_tasks';
      userReference: string | null;
    }
  | {
      channelReference: string | null;
      enabled: boolean;
      name: 'set_ingestion_policy';
    }
  | {
      channelReference: string | null;
      name: 'get_ingestion_status';
    }
  | {
      affectedRoleReferences: string[];
      channelReference: string | null;
      description: string;
      dueAt: string | null;
      name: 'create_assignment';
      title: string;
    }
  | {
      name: 'list_assignments';
    }
  | {
      assignmentReference: string;
      channelReference: string | null;
      name: 'publish_assignment';
    }
  | {
      capability: string;
      name: 'grant_permission';
      userReference: string;
    }
  | {
      capability: string;
      name: 'revoke_permission';
      userReference: string;
    }
  | {
      name: 'list_permissions';
      userReference: string | null;
    }
  | {
      context: string | null;
      message: string;
      name: 'send_dm_relay';
      recipientReference: string;
    };

export interface ToolPlan {
  toolCalls: PlannedToolCall[];
}

export interface ToolPlanningClient {
  planDmTools(input: {
    instructions: string;
    model: string;
    text: string;
  }): Promise<ToolPlan>;
}
