import type { SchemaTemplate } from './types';

// Preset templates for common entity types
export const SCHEMA_TEMPLATES: Record<string, SchemaTemplate> = {
  player: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Player ID', description: 'Unique player identifier' },
      name: { type: 'string', title: 'Name', description: 'Player display name' },
      level: {
        type: 'integer',
        title: 'Level',
        description: 'Player level',
        minimum: 1,
        default: 1,
      },
      experience: {
        type: 'number',
        title: 'Experience',
        description: 'Total experience points',
        default: 0,
      },
      coins: { type: 'integer', title: 'Coins', description: 'In-game currency', default: 0 },
      gems: { type: 'integer', title: 'Gems', description: 'Premium currency', default: 0 },
      isVip: {
        type: 'boolean',
        title: 'VIP Status',
        description: 'Whether player has VIP',
        default: false,
      },
      lastLogin: { type: 'string', title: 'Last Login', format: 'date-time' },
      serverId: { type: 'string', title: 'Server ID', description: 'Home server identifier' },
    },
    required: ['id', 'name'],
  },
  item: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Item ID', description: 'Unique item identifier' },
      name: { type: 'string', title: 'Name', description: 'Item name' },
      type: {
        type: 'string',
        title: 'Type',
        enum: ['weapon', 'armor', 'consumable', 'material', 'special'],
      },
      rarity: {
        type: 'string',
        title: 'Rarity',
        enum: ['common', 'uncommon', 'rare', 'epic', 'legendary'],
      },
      level: { type: 'integer', title: 'Required Level', minimum: 1, default: 1 },
      price: { type: 'number', title: 'Price', description: 'Base price in coins', minimum: 0 },
      stackable: { type: 'boolean', title: 'Stackable', default: false },
      maxStack: { type: 'integer', title: 'Max Stack Size', minimum: 1, default: 99 },
    },
    required: ['id', 'name', 'type'],
  },
  guild: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'Guild ID', description: 'Unique guild identifier' },
      name: { type: 'string', title: 'Guild Name', description: 'Display name' },
      leader: { type: 'string', title: 'Leader ID', description: 'Guild leader player ID' },
      level: { type: 'integer', title: 'Guild Level', minimum: 1, default: 1 },
      members: {
        type: 'array',
        title: 'Member List',
        description: 'Array of member IDs',
        items: { type: 'string' },
      },
      maxMembers: { type: 'integer', title: 'Max Members', minimum: 1, default: 50 },
      description: { type: 'string', title: 'Description', description: 'Guild description' },
      created: { type: 'string', title: 'Created At', format: 'date-time' },
    },
    required: ['id', 'name', 'leader'],
  },
  basic: {
    type: 'object',
    properties: {
      id: { type: 'string', title: 'ID', description: 'Unique identifier' },
      name: { type: 'string', title: 'Name', description: 'Display name' },
      description: { type: 'string', title: 'Description', description: 'Detailed description' },
      active: { type: 'boolean', title: 'Active', default: true },
    },
    required: ['id', 'name'],
  },
};
