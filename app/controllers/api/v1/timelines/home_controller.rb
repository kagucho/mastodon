# frozen_string_literal: true

class Api::V1::Timelines::HomeController < Api::BaseController
  before_action -> { doorkeeper_authorize! :read }, only: [:show]
  before_action :require_user!, only: [:show]
  after_action :insert_pagination_headers, unless: -> { @statuses.empty? }

  respond_to :json

  def show
    @statuses = load_statuses
    render json: @statuses, each_serializer: REST::StatusSerializer, relationships: StatusRelationshipsPresenter.new(@statuses, current_user&.account_id)
  end

  private

  # An approximation of the number of statuses per day
  RANGE_REQUEST_MAX_ID_RANGE = 262_144

  def load_statuses
    cached_home_statuses
  end

  def cached_home_statuses
    cache_collection home_statuses, Status
  end

  def home_statuses
    max_id = params[:max_id]&.to_i
    since_id = params[:since_id]&.to_i

    if max_id.nil? && since_id.nil?
      since_id = Status.first.id - FeedManager::MIN_ID_RANGE
    elsif max_id.nil? && since_id.present?
      max_id = since_id + RANGE_REQUEST_MAX_ID_RANGE
    elsif max_id.present? && since_id.nil?
      since_id = max_id - RANGE_REQUEST_MAX_ID_RANGE
    elsif max_id - since_id > RANGE_REQUEST_MAX_ID_RANGE
      raise Mastodon::ValidationError, 'Too broad range for ID'
    end

    account_home_feed.get(
      limit_param(DEFAULT_STATUSES_LIMIT),
      max_id,
      since_id
    )
  end

  def account_home_feed
    Feed.new(:home, current_account)
  end

  def insert_pagination_headers
    set_pagination_headers(next_path, prev_path)
  end

  def pagination_params(core_params)
    params.permit(:local, :limit).merge(core_params)
  end

  def next_path
    api_v1_timelines_home_url pagination_params(max_id: pagination_max_id)
  end

  def prev_path
    api_v1_timelines_home_url pagination_params(since_id: pagination_since_id)
  end

  def pagination_max_id
    @statuses.last.id
  end

  def pagination_since_id
    @statuses.first.id
  end
end
