# frozen_string_literal: true

class DestructAccountService < BaseService
  def call(account, **options)
    @account = account
    @account_already_suspended = account.suspended?
    @options = options

    purge_user!
    purge_profile!
    purge_content!
    unsubscribe_push_subscribers!
  end

  private

  def purge_user!
    if @options[:remove_user]
      @account.user&.destroy
    else
      @account.user&.disable!
    end
  end

  def purge_content!
    if @account_already_suspended
      @account.statuses.reorder(nil).in_batches.destroy_all
    else
      ActivityPub::RawDistributionWorker.perform_async(delete_actor_json, @account.id) if @account.local?

      @account.statuses.reorder(nil).find_in_batches do |statuses|
        BatchedRemoveAccountStatusService.new.call(statuses)
      end
    end

    [
      @account.media_attachments,
      @account.stream_entries,
      @account.notifications,
      @account.favourites,
      @account.active_relationships,
      @account.passive_relationships,
    ].each do |association|
      destroy_all(association)
    end
  end

  def purge_profile!
    @account.suspended    = true
    @account.terminated   = true
    @account.display_name = ''
    @account.note         = ''
    @account.avatar.destroy
    @account.header.destroy
    @account.save!
  end

  def unsubscribe_push_subscribers!
    destroy_all(@account.subscriptions)
  end

  def destroy_all(association)
    association.in_batches.destroy_all
  end

  def delete_actor_json
    payload = ActiveModelSerializers::SerializableResource.new(
      @account,
      serializer: ActivityPub::DeleteActorSerializer,
      adapter: ActivityPub::Adapter
    ).as_json

    Oj.dump(ActivityPub::LinkedDataSignature.new(payload).sign!(@account))
  end
end
