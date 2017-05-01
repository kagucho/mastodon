class CreateAccountRelations < ActiveRecord::Migration[5.0]
  def change
    reversible do |dir|
      dir.up do
        execute "CREATE TYPE account_relation_type AS ENUM ('block', 'mute')"
      end

      dir.down do
        execute 'DROP TYPE account_relation_type'
      end
    end

    create_table 'account_relations', id: false, force: :casecade do |t|
      t.integer  'id',                            null: false
      t.integer  'account_id',                    null: false
      t.integer  'target_account_id',             null: false
      t.datetime 'created_at',                    null: false
      t.datetime 'updated_at',                    null: false
      t.column   'type', 'account_relation_type', null: false
    end

    reversible do |dir|
      dir.up do
        add_column       :blocks, :type, 'account_relation_type'
        Block.update_all type: :block
        change_column    :blocks, :type, 'account_relation_type', null: false
        execute          "ALTER TABLE blocks ADD CONSTRAINT check_blocks_on_type CHECK (type = 'block'), INHERIT account_relations"

        add_column       :mutes, :type, 'account_relation_type'
        Mute.update_all  type: :mute
        change_column    :mutes, :type, 'account_relation_type', null: false
        execute          "ALTER TABLE mutes ADD CONSTRAINT check_mutes_on_type CHECK (type = 'mute'), INHERIT account_relations"
      end

      dir.down do
        execute       "ALTER TABLE blocks NO INHERIT account_relations, DROP CONSTRAINT check_blocks_on_type"
        remove_column :blocks, :type

        execute       "ALTER TABLE mutes NO INHERIT account_relations, DROP CONSTRAINT check_mutes_on_type"
        remove_column :mutes, :type
      end
    end
  end
end
